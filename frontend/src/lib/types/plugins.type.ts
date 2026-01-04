export interface PluginState {
    id: string;
    environment_id: string;
    plugin_id: string;
    base_url: string;
    enabled: boolean;
    config: Record<string, any>;
    created_at: string;
    updated_at?: string;
}

export interface PluginManifest {
    plugin_id: string;
    name: string;
    version: string;
    capabilities: string[];
    ui: PluginUIConfig;
}

export interface PluginUIConfig {
    nav?: PluginNavItem[];
    project_tabs?: PluginProjectTab[];
    settings_page?: string;
}

export interface PluginNavItem {
    label: string;
    path: string;
    icon?: string;
}

export interface PluginProjectTab {
    label: string;
    path: string;
}
