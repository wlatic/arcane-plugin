package handlers

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/getarcaneapp/arcane/backend/internal/models"
	"github.com/getarcaneapp/arcane/backend/internal/services"
)

type RegisterPluginInput struct {
	EnvironmentID string `path:"id" doc:"Environment ID"`
	Body          struct {
		PluginID string `json:"plugin_id" doc:"Unique identifier for the plugin"`
		BaseURL  string `json:"base_url" doc:"Base URL where the plugin service is running"`
	}
}

type RegisterPluginOutput struct {
	Body struct {
		Plugin *models.PluginState `json:"plugin"`
	}
}

type ListPluginsInput struct {
	EnvironmentID string `path:"id" doc:"Environment ID"`
}

type ListPluginsOutput struct {
	Body struct {
		Plugins []models.PluginState `json:"plugins"`
	}
}

func RegisterPlugins(api huma.API, registry *services.PluginRegistryService) {
	huma.Register(api, huma.Operation{
		OperationID: "register-plugin",
		Method:      http.MethodPost,
		Path:        "/api/environments/{id}/plugins/register",
		Summary:     "Register a plugin",
		Tags:        []string{"Plugins"},
	}, func(ctx context.Context, input *RegisterPluginInput) (*RegisterPluginOutput, error) {
		plugin, err := registry.RegisterPlugin(input.EnvironmentID, input.Body.PluginID, input.Body.BaseURL)
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to register plugin", err)
		}

		return &RegisterPluginOutput{
			Body: struct {
				Plugin *models.PluginState `json:"plugin"`
			}{
				Plugin: plugin,
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-plugins",
		Method:      http.MethodGet,
		Path:        "/api/environments/{id}/plugins",
		Summary:     "List plugins",
		Tags:        []string{"Plugins"},
	}, func(ctx context.Context, input *ListPluginsInput) (*ListPluginsOutput, error) {
		plugins, err := registry.ListPlugins(input.EnvironmentID)
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to list plugins", err)
		}
		return &ListPluginsOutput{
			Body: struct {
				Plugins []models.PluginState `json:"plugins"`
			}{
				Plugins: plugins,
			},
		}, nil
	})
}

// RegisterPluginProxy registers a wildcard route on the Gin router to proxy requests to plugins
func RegisterPluginProxy(router *gin.RouterGroup, registry *services.PluginRegistryService) {
	// Match /api/environments/:id/plugins/:pluginId/*path
	router.Any("/environments/:id/plugins/:pluginId/*path", func(c *gin.Context) {
		envID := c.Param("id")
		pluginID := c.Param("pluginId")
		proxyPath := c.Param("path")

		// Lookup Plugin
		plugin, err := registry.GetPlugin(envID, pluginID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
			return
		}

		if !plugin.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "Plugin is disabled"})
			return
		}

		targetURL, err := url.Parse(plugin.BaseURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid plugin BaseURL"})
			return
		}

		// Create Proxy
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			// Rewrite Path
			// User Spec: "/api/plugins/:pluginId/*" -> "base_url/api/*"
			// Here we are treating everything under this route as API because it's in the API group.

			req.URL.Path = singleJoiningSlash("/api", proxyPath)

			// Create JWT
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"sub": "arcane-core",
				"env": envID,
				"iat": time.Now().Unix(),
				"exp": time.Now().Add(5 * time.Minute).Unix(),
			})

			// Sign with shared secret
			tokenString, _ := token.SignedString([]byte(plugin.SharedSecret))
			req.Header.Set("X-Arcane-Plugin-Auth", tokenString)
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	})
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
