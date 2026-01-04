package services

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/getarcaneapp/arcane/backend/internal/database"
	"github.com/getarcaneapp/arcane/backend/internal/models"
	"github.com/getarcaneapp/arcane/backend/internal/utils"
	"github.com/getarcaneapp/arcane/types/project"
)

type GitService struct {
	settings     *SettingsService
	db           *database.DB
	gitPath      string
	projectLocks sync.Map      // map[string]*sync.Mutex (Key: ProjectID)
	semaphore    chan struct{} // Buffered channel to limit concurrent git operations
}

func NewGitService(db *database.DB, settings *SettingsService) *GitService {
	s := &GitService{
		settings:  settings,
		db:        db,
		semaphore: make(chan struct{}, 5), // Global limit of 5 concurrent git operations
	}
	s.gitPath = s.resolveGitPath()
	return s
}

func (s *GitService) resolveGitPath() string {
	// 1. Try PATH
	if path, err := exec.LookPath("git"); err == nil {
		slog.Info("Git found in PATH", "path", path)
		return path
	}

	// 2. Common Windows Paths
	commonPaths := []string{
		`C:\Program Files\Git\cmd\git.exe`,
		`C:\Program Files\Git\bin\git.exe`,
		`C:\Program Files (x86)\Git\cmd\git.exe`,
		`C:\Program Files (x86)\Git\bin\git.exe`,
	}

	// Add User AppData paths
	home, err := os.UserHomeDir()
	if err == nil {
		commonPaths = append(commonPaths,
			filepath.Join(home, "AppData", "Local", "Programs", "Git", "cmd", "git.exe"),
		)

		// GitHub Desktop (wildcard)
		ghPattern := filepath.Join(home, "AppData", "Local", "GitHubDesktop", "app-*", "resources", "app", "git", "cmd", "git.exe")
		matches, _ := filepath.Glob(ghPattern)
		if len(matches) > 0 {
			// Use the latest version? Glob usually returns sorted?
			// app-3.5.3 vs app-3.4.0.
			// Let's just pick the last one (highest version usually)
			commonPaths = append(commonPaths, matches[len(matches)-1])
		}
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			slog.Info("Git found in common path", "path", p)
			return p
		}
	}

	slog.Warn("Git executable not found in PATH or common locations. Git operations may fail.")
	return "git" // Fallback
}

func (s *GitService) Sync(ctx context.Context, project *models.Project) (string, error) {
	return s.RunGitOp(ctx, project.ID, "Sync", func(ctx context.Context) (string, error) {
		slog.InfoContext(ctx, "Syncing project from git", "project_id", project.ID, "project_name", project.Name)

		repoURL, authUser, authToken := s.getEffectiveGitConfig(project)
		if repoURL == "" {
			return "", fmt.Errorf("git repository not configured")
		}

		authURL, err := getAuthenticatedURL(repoURL, authUser, authToken)
		if err != nil {
			return "", err
		}

		projectDir := getProjectDir(project)
		if projectDir == "" {
			return "", fmt.Errorf("project directory not found")
		}

		// Init if needed
		if err := s.initRepoIfNeeded(ctx, projectDir); err != nil {
			return "", err
		}

		// Fetch
		if err := s.runGitCommand(ctx, projectDir, "fetch", authURL); err != nil {
			return "", fmt.Errorf("failed to git fetch: %w", err)
		}

		// Check if we are behind
		branch := s.getEffectiveBranch(project, projectDir)
		if branch == "" {
			branch = "main" // fallback
		}

		_, behind, err := s.getSyncCounts(ctx, projectDir, branch)
		countsAvailable := err == nil
		if err != nil {
			slog.WarnContext(ctx, "Failed to check sync counts, proceeding with pull anyway", "error", err)
		} else if behind == 0 {
			slog.InfoContext(ctx, "Project is already up to date", "project_id", project.ID)
			// Update last sync time anyway to show we checked
			now := time.Now()
			project.GitLastSyncAt = &now
			s.db.WithContext(ctx).Model(project).Update("git_last_sync_at", now)
			return "Already up to date", nil
		}

		slog.InfoContext(ctx, "Project is behind remote, pulling changes", "project_id", project.ID, "behind_count", behind)

		// 1. Check for Dirty Tree
		statusOut, err := s.runGitCommandWithOutput(ctx, projectDir, "status", "--porcelain")
		if err != nil {
			return "", fmt.Errorf("failed to check git status: %w", err)
		}
		if strings.TrimSpace(statusOut) != "" {
			return "", fmt.Errorf("working tree is dirty: uncommitted changes present (commit or stash them)")
		}

		// 2. Pull with --ff-only (Safe Update)
		cmdArgs := []string{"pull", "--ff-only", authURL, "HEAD"}
		// Allow unrelated histories if we just initialized (and it's still FF safe)?
		// --ff-only will reject unrelated histories usually.
		// If we want to support initial sync of empty local repo -> remote, we might need flexibility.
		// But "production hardening" implies strict safety. Let's start strict.

		output, pullErr := s.runGitCommandWithOutput(ctx, projectDir, cmdArgs...)
		if pullErr != nil {
			// Check for non-fast-forward / conflict
			// Check for "fatal: Not possible to fast-forward, aborting."
			// Or general failure.
			// Mark project as Conflict
			slog.WarnContext(ctx, "Git Pull failed (non-ff?)", "error", pullErr)

			// Update status to Conflict
			if err := s.db.WithContext(ctx).Model(project).Update("status", models.ProjectStatusGitConflict).Error; err != nil {
				slog.WarnContext(ctx, "Failed to update project status to conflict", "error", err)
			}

			if strings.Contains(pullErr.Error(), "fast-forward") || strings.Contains(pullErr.Error(), "conflict") {
				return "", fmt.Errorf("git conflict: branch diverged, cannot auto-pull (requires manual intervention)")
			}
			return "", fmt.Errorf("failed to git pull: %w", pullErr)
		}

		// Update Last Sync Time & Clear Conflict Status if successful?
		// We rely on standard polling to correct status if it was conflict.

		now := time.Now()
		project.GitLastSyncAt = &now
		if err := s.db.WithContext(ctx).Model(project).Update("git_last_sync_at", now).Error; err != nil {
			slog.WarnContext(ctx, "Failed to update git_last_sync_at", "error", err)
		}

		if strings.Contains(output, "Already up to date") {
			return "Already up to date", nil
		}

		if countsAvailable {
			return fmt.Sprintf("Successfully pulled %d commits", behind), nil
		}
		return "Successfully pulled changes", nil
	})
}

func (s *GitService) Push(ctx context.Context, project *models.Project, message string) (string, error) {
	return s.RunGitOp(ctx, project.ID, "Push", func(ctx context.Context) (string, error) {
		slog.InfoContext(ctx, "Pushing project to git", "project_id", project.ID, "project_name", project.Name)

		repoURL, authUser, authToken := s.getEffectiveGitConfig(project)
		if repoURL == "" {
			return "", fmt.Errorf("git repository not configured")
		}

		projectDir := getProjectDir(project)
		if projectDir == "" {
			return "", fmt.Errorf("project directory not found")
		}

		// Init if needed
		if err := s.initRepoIfNeeded(ctx, projectDir); err != nil {
			return "", err
		}

		// Stage all changes
		if err := s.runGitCommand(ctx, projectDir, "add", "."); err != nil {
			return "", fmt.Errorf("failed to git add: %w", err)
		}

		// Commit
		// Check if there are changes first? git diff --cached --quiet
		committed := false
		if err := s.runGitCommand(ctx, projectDir, "diff", "--cached", "--quiet"); err != nil {
			// If error is exit code 1, it means there are diffs.
			commitArgs := []string{"commit", "-m", message}
			if authUser != "" {
				commitArgs = append(commitArgs, "--author", fmt.Sprintf("%s <%s@noreply.github.com>", authUser, authUser))
			}

			if err := s.runGitCommand(ctx, projectDir, commitArgs...); err != nil {
				return "", fmt.Errorf("failed to git commit: %w", err)
			}
			committed = true
		}

		authURL, err := getAuthenticatedURL(repoURL, authUser, authToken)
		if err != nil {
			return "", err
		}

		// Enforce the configured branch name if set
		targetBranch := s.getEffectiveBranch(project, projectDir)
		if targetBranch != "" {
			s.ensureBranch(ctx, projectDir, targetBranch)
		}

		// Fetch first to ensure we have latest remote refs for comparison
		if err := s.runGitCommand(ctx, projectDir, "fetch", authURL); err != nil {
			slog.WarnContext(ctx, "Failed to fetch before push check, proceeding", "error", err)
		}

		// Check if ahead
		if targetBranch != "" {
			ahead, _, err := s.getSyncCounts(ctx, projectDir, targetBranch)
			if err == nil {
				if ahead == 0 {
					slog.InfoContext(ctx, "Nothing to push (ahead is 0)", "project_id", project.ID)
					if committed {
						return "Committed changes, but nothing to push (remote matches?)", nil
					}
					return "Nothing to push", nil
				}
				slog.InfoContext(ctx, "Project is ahead of remote, pushing", "project_id", project.ID, "ahead_count", ahead)
			} else {
				slog.WarnContext(ctx, "Failed to check sync counts, proceeding with push check", "error", err)
			}
		}

		// Push
		cmdArgs := []string{"push", "--set-upstream", authURL, "HEAD"}

		output, err := s.runGitCommandWithOutput(ctx, projectDir, cmdArgs...)
		if err != nil {
			// Removed auto-sync logic for safety.
			if strings.Contains(err.Error(), "non-fast-forward") || strings.Contains(err.Error(), "Updates were rejected") {
				return "", fmt.Errorf("push failed: remote contains work that you do not have locally (diverged). Please pull first.")
			}
			return "", fmt.Errorf("failed to git push: %w", err)
		}

		if strings.Contains(output, "Everything up-to-date") {
			if committed {
				return "Committed changes, but nothing to push (remote matches?)", nil
			}
			return "Nothing to push", nil
		}

		return "Successfully pushed changes", nil
	})
}

func (s *GitService) ensureBranch(ctx context.Context, dir, branch string) {
	// Simple rename of current branch to target if we're on valid HEAD
	// Or checkout? For now stick to simple rename if we are just ensuring we push to "main" or configured branch
	currentBranch, _ := s.getCurrentBranch(ctx, dir)
	if currentBranch != "" && currentBranch != branch {
		slog.Info("Renaming local branch to match configuration", "from", currentBranch, "to", branch)
		if err := s.runGitCommand(ctx, dir, "branch", "-M", branch); err != nil {
			slog.Warn("Failed to rename branch to match config", "error", err)
		}
	}
}

func (s *GitService) initRepoIfNeeded(ctx context.Context, dir string) error {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return nil // already initialized
	}

	slog.Info("Initializing git repository", "dir", dir)
	if err := s.runGitCommand(ctx, dir, "init"); err != nil {
		return fmt.Errorf("failed to git init: %w", err)
	}

	if err := s.runGitCommand(ctx, dir, "branch", "-M", "main"); err != nil {
		slog.Warn("Failed to rename default branch to main", "error", err)
	}

	return nil
}

func (s *GitService) Status(ctx context.Context, project *models.Project) (string, error) {
	return s.RunGitOp(ctx, project.ID, "Status", func(ctx context.Context) (string, error) {
		projectDir := getProjectDir(project)
		if projectDir == "" {
			return "", fmt.Errorf("project directory not found")
		}

		repoURL, authUser, authToken := s.getEffectiveGitConfig(project)
		if repoURL == "" {
			return "not_configured", nil
		}

		authURL, err := getAuthenticatedURL(repoURL, authUser, authToken)
		if err != nil {
			return "", err
		}

		if err := s.runGitCommand(ctx, projectDir, "fetch", authURL); err != nil {
			return "fetch_failed", nil
		}

		branch := s.getEffectiveBranch(project, projectDir)
		if branch == "" {
			out, err := exec.CommandContext(ctx, s.gitPath, "-C", projectDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
			if err == nil {
				branch = strings.TrimSpace(string(out))
			} else {
				branch = "main"
			}
		}

		cmd := exec.CommandContext(ctx, s.gitPath, "-C", projectDir, "rev-list", "--left-right", "--count", "HEAD...origin/"+branch)
		out, err := cmd.Output()
		if err != nil {
			return "unknown", nil
		}

		counts := strings.Fields(string(out))
		if len(counts) != 2 {
			return "unknown", nil
		}

		ahead := counts[0]
		behind := counts[1]

		if ahead == "0" && behind == "0" {
			return "synced", nil
		}

		status := ""
		if ahead != "0" {
			status += fmt.Sprintf("Ahead %s", ahead)
		}
		if behind != "0" {
			if status != "" {
				status += ", "
			}
			status += fmt.Sprintf("Behind %s", behind)
		}

		return status, nil
	})
}

func (s *GitService) getSyncCounts(ctx context.Context, projectDir, branch string) (int, int, error) {
	cmd := exec.CommandContext(ctx, s.gitPath, "rev-list", "--left-right", "--count", "HEAD...origin/"+branch)
	cmd.Dir = projectDir
	// Inherit env to ensure git works if needed, though rev-list is local usually (unless it needs to read config)
	cmd.Env = os.Environ()

	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	counts := strings.Fields(string(out))
	if len(counts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %s", string(out))
	}

	ahead := 0
	behind := 0
	fmt.Sscanf(counts[0], "%d", &ahead)
	fmt.Sscanf(counts[1], "%d", &behind)

	return ahead, behind, nil
}

func (s *GitService) CheckAndSyncAll(ctx context.Context) error {
	var projects []models.Project
	if err := s.db.WithContext(ctx).Find(&projects).Error; err != nil {
		return err
	}

	for _, p := range projects {
		if p.GitSyncMode == "pull" || p.GitSyncMode == "push" {
			// Determine interval
			interval := p.GitPollInterval
			if interval <= 0 {
				interval = 5 // Default
			}

			// Check eligibility
			shouldSync := false
			if p.GitLastSyncAt == nil {
				shouldSync = true
			} else {
				nextSync := p.GitLastSyncAt.Add(time.Duration(interval) * time.Minute)
				if time.Now().After(nextSync) {
					shouldSync = true
				}
			}

			if shouldSync {
				if p.GitSyncMode == "pull" {
					if msg, err := s.Sync(ctx, &p); err != nil {
						slog.Warn("Auto-sync (pull) failed", "project", p.Name, "error", err)
					} else {
						slog.Info("Auto-sync (pull) completed", "project", p.Name, "message", msg)
					}
				} else if p.GitSyncMode == "push" {
					msgContent := fmt.Sprintf("Auto-save: %s", time.Now().Format(time.RFC3339))
					if msg, err := s.Push(ctx, &p, msgContent); err != nil {
						slog.Warn("Auto-sync (push) failed", "project", p.Name, "error", err)
					} else {
						slog.Info("Auto-sync (push) completed", "project", p.Name, "message", msg)
						// Update last sync time on successful push too
						now := time.Now()
						p.GitLastSyncAt = &now
						if err := s.db.WithContext(ctx).Model(&p).Update("git_last_sync_at", now).Error; err != nil {
							slog.Warn("Failed to update git_last_sync_at", "error", err)
						}
					}
				}
			}
		}
	}
	return nil
}

// GetLog returns the recent git commit history
func (s *GitService) GetLog(ctx context.Context, p *models.Project) ([]project.GitLogEntry, error) {
	var logs []project.GitLogEntry
	_, err := s.RunGitOp(ctx, p.ID, "GetLog", func(ctx context.Context) (string, error) {
		projectDir := getProjectDir(p)
		if _, err := os.Stat(projectDir); os.IsNotExist(err) {
			return "", fmt.Errorf("project directory does not exist")
		}

		// Double check it's a git repo
		if err := s.initRepoIfNeeded(ctx, projectDir); err != nil {
			return "", err
		}

		// git log -n 20 --pretty=format:"%h|%an|%ar|%s"
		out, err := s.runGitCommandWithOutput(ctx, projectDir, "log", "-n", "20", "--pretty=format:%h|%an|%ar|%s")
		if err != nil {
			return "", fmt.Errorf("failed to get git log: %w", err)
		}

		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				logs = append(logs, project.GitLogEntry{
					Hash:    parts[0],
					Author:  parts[1],
					Date:    parts[2],
					Message: strings.Join(parts[3:], "|"), // Re-join in case message contained separator
				})
			}
		}
		return "", nil
	})
	return logs, err
}

// Helpers

type limitWriter struct {
	w *bytes.Buffer
	n int
}

func (l *limitWriter) Write(p []byte) (n int, err error) {
	if l.w.Len() >= l.n {
		return len(p), nil // discard
	}
	limit := l.n - l.w.Len()
	if len(p) > limit {
		p = p[:limit]
	}
	return l.w.Write(p)
}

func (s *GitService) sanitizeLogOutput(output string) string {
	// Mask credentials in URLs: https://user:token@host -> https://***:***@host
	re := regexp.MustCompile(`(https?://)([^:]+):([^@]+)@`)
	return re.ReplaceAllString(output, "${1}***:***@")
}

func (s *GitService) RunGitOp(ctx context.Context, lockKey string, opName string, fn func(ctx context.Context) (string, error)) (string, error) {
	// 1. Acquire Per-Project Lock (if key provided)
	if lockKey != "" {
		lock, _ := s.projectLocks.LoadOrStore(lockKey, &sync.Mutex{})
		mu := lock.(*sync.Mutex)
		mu.Lock()
		defer mu.Unlock()
	}

	// 2. Acquire Global Semaphore
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// 3. Apply Timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 4. Run Operation
	return fn(ctx)
}

func (s *GitService) runGitCommand(ctx context.Context, dir string, args ...string) error {
	_, err := s.runGitCommandWithOutput(ctx, dir, args...)
	return err
}

func (s *GitService) runGitCommandWithOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, s.gitPath, args...)
	cmd.Dir = dir
	// Prevent password prompts blocking
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &limitWriter{w: &stdoutBuf, n: 4096} // Limit output to 4KB
	cmd.Stderr = &limitWriter{w: &stderrBuf, n: 4096}

	if err := cmd.Run(); err != nil {
		sanitizedErr := s.sanitizeLogOutput(stderrBuf.String())
		// If explicit error output is empty, try to use stdout as well just in case
		if sanitizedErr == "" {
			sanitizedErr = s.sanitizeLogOutput(stdoutBuf.String())
		}
		if sanitizedErr == "" {
			sanitizedErr = err.Error()
		}

		// Sanitize args for logging
		sanitizedArgs := make([]string, len(args))
		for i, arg := range args {
			sanitizedArgs[i] = s.sanitizeLogOutput(arg)
		}

		slog.ErrorContext(ctx, "Git command failed", "args", sanitizedArgs, "error", err, "stderr", sanitizedErr)
		return "", fmt.Errorf("git error: %s", sanitizedErr)
	}

	return s.sanitizeLogOutput(stdoutBuf.String()), nil
}

func (s *GitService) getEffectiveGitConfig(p *models.Project) (repoURL, user, token string) {
	// 1. Repo URL (Project override)
	if p.GitRepoURL != nil && *p.GitRepoURL != "" {
		repoURL = *p.GitRepoURL
	}

	// 2. Authentication (Identity vs Manual)
	if p.GitIdentityID != nil && p.GitIdentity != nil {
		user = p.GitIdentity.Username
		decryptedToken, err := utils.Decrypt(p.GitIdentity.Token)
		if err == nil {
			token = decryptedToken
		} else {
			slog.Error("Failed to decrypt git identity token", "error", err, "identityID", p.GitIdentity.ID)
		}
	} else {
		// Manual fields
		if p.GitAuthUser != nil && *p.GitAuthUser != "" {
			user = *p.GitAuthUser
		}
		if p.GitAuthToken != nil && *p.GitAuthToken != "" {
			// Try to decrypt; if it fails, assume it's legacy cleartext (or invalid ciphertext that looks like cleartext)
			decrypted, err := utils.Decrypt(*p.GitAuthToken)
			if err == nil {
				token = decrypted
			} else {
				// Fallback to raw value (for migration period or if encryption failed on save somehow)
				token = *p.GitAuthToken
			}
		}
	}

	// 3. Global defaults (lowest priority)
	cfg := s.settings.GetSettingsConfig()
	if repoURL == "" {
		repoURL = cfg.GitGlobalRepo.Value
	}
	// Only apply global defaults if we haven't found specific creds
	if user == "" {
		user = cfg.GitGlobalUser.Value
	}
	if token == "" {
		token = cfg.GitGlobalToken.Value
	}

	return
}

func (s *GitService) getEffectiveBranch(p *models.Project, dir string) string {
	if p.GitBranch != nil && *p.GitBranch != "" {
		return *p.GitBranch
	}

	// Try to detect current branch from local repo
	if dir != "" {
		if branch, err := s.getCurrentBranch(context.Background(), dir); err == nil && branch != "" {
			return branch
		}
	}

	return s.settings.GetSettingsConfig().GitGlobalBranch.Value
}

func (s *GitService) getCurrentBranch(ctx context.Context, dir string) (string, error) {
	// git branch --show-current
	cmd := exec.CommandContext(ctx, s.gitPath, "branch", "--show-current")
	cmd.Dir = dir
	cmd.Env = os.Environ() // Ensure PATH is available

	out, err := cmd.Output()
	if err != nil {
		slog.Warn("Failed to get current branch", "dir", dir, "error", err)
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	slog.Info("Detected current branch", "branch", branch, "dir", dir)
	return branch, nil
}

func getAuthenticatedURL(repoURL, user, token string) (string, error) {
	if token == "" {
		return repoURL, nil
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}

	if user != "" {
		u.User = url.UserPassword(user, token)
	} else {
		u.User = url.User(token) // Some services just use token as user
	}

	return u.String(), nil
}

func getProjectDir(p *models.Project) string {
	if p.DirName != nil && *p.DirName != "" {
		// This might need resolving dependent on how projects are stored.
		// Assuming p.Path is absolute or resolved.
		// Let's check project struct again.
		// It has `Path`.
		return p.Path
	}
	return p.Path
}

func (s *GitService) ValidateConnection(ctx context.Context, repoURL, branch, user, token string) error {
	slog.InfoContext(ctx, "Validating git connection", "repoURL", repoURL, "branch", branch, "user", user)

	_, err := s.RunGitOp(ctx, "", "ValidateConnection", func(ctx context.Context) (string, error) {
		authURL, err := getAuthenticatedURL(repoURL, user, token)
		if err != nil {
			return "", fmt.Errorf("invalid repository URL: %w", err)
		}

		// Use ls-remote to check access without cloning
		// git ls-remote --heads <url> <branch>
		args := []string{"ls-remote", "--heads", authURL}
		if branch != "" {
			args = append(args, branch)
		}

		cmd := exec.CommandContext(ctx, s.gitPath, args...)
		cmd.Dir = os.TempDir()
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")

		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &limitWriter{w: &stdoutBuf, n: 4096}
		cmd.Stderr = &limitWriter{w: &stderrBuf, n: 4096}

		if err := cmd.Run(); err != nil {
			sanitizedErr := s.sanitizeLogOutput(stderrBuf.String())
			if sanitizedErr == "" {
				sanitizedErr = s.sanitizeLogOutput(stdoutBuf.String())
			}
			if sanitizedErr == "" {
				sanitizedErr = err.Error()
			}

			if strings.Contains(sanitizedErr, "Authentication failed") || strings.Contains(sanitizedErr, "Invalid username or password") {
				return "", fmt.Errorf("authentication failed")
			}
			if strings.Contains(sanitizedErr, "Repository not found") {
				return "", fmt.Errorf("repository not found")
			}
			return "", fmt.Errorf("validation failed: %s", sanitizedErr)
		}

		// If branch was specified, check if it was returned
		if branch != "" {
			outStr := s.sanitizeLogOutput(stdoutBuf.String())
			if strings.TrimSpace(outStr) == "" {
				return "", fmt.Errorf("branch '%s' not found in remote", branch)
			}
		}

		return "", nil
	})
	return err
}
