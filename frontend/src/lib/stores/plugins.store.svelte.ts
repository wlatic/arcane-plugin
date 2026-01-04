import type { PluginState, PluginManifest, PluginNavItem } from '$lib/types/plugins.type';
import { environmentStore } from './environment.store.svelte';

function createPluginsStore() {
    let _plugins = $state<PluginState[]>([]);
    let _manifests = $state<Record<string, PluginManifest>>({});

    return {
        get plugins() {
            return _plugins;
        },
        get manifests() {
            return _manifests;
        },
        get sidebarItems() {
            const items: (PluginNavItem & { pluginId: string; href: string })[] = [];
            for (const [id, m] of Object.entries(_manifests)) {
                if (m.ui?.nav) {
                    for (const item of m.ui.nav) {
                        // Construct the internal SvelteKit URL for the plugin view
                        // Route: /plugins/[pluginId]/[...path]
                        const cleanPath = item.path.startsWith('/') ? item.path.slice(1) : item.path;
                        items.push({
                            ...item,
                            pluginId: id,
                            href: `/plugins/${id}/${cleanPath}`
                        });
                    }
                }
            }
            return items;
        },
        loadPlugins: async (envId: string) => {
            try {
                const res = await fetch(`/api/environments/${envId}/plugins`);
                if (res.ok) {
                    const data = await res.json();
                    // Handle different response structures if wrapped
                    const list = data.body?.plugins || data.data?.plugins || data.plugins || [];
                    _plugins = list;

                    // Load manifests for enabled plugins
                    for (const p of _plugins) {
                        if (p.enabled) {
                            // Check if we already have it? Reloading is safer for dev
                            loadManifest(envId, p.plugin_id);
                        }
                    }
                }
            } catch (e) {
                console.error('Failed to load plugins', e);
            }
        }
    };
}

async function loadManifest(envId: string, pluginId: string) {
    try {
        // Use the Proxy to fetch manifest from the Plugin Service
        // GET /api/environments/:id/plugins/:pluginId/manifest
        // Note: The Plugin Service exposes /manifest. 
        // The Proxy maps /api/environments/id/plugins/pluginId/* -> base_url/api/* ?
        // Wait, my Proxy Logic:
        // "/api/environments/:id/plugins/:pluginId/*path" -> "base_url/api/*path" NO
        // I implemented: "base_url/api" + proxyPath.
        // If Plugin exposes `/manifest` at root...
        // And Proxy mounts at `/api/...`.
        // If I request `/api/environments/x/plugins/y/manifest`, it goes to `base_url/api/manifest`.
        // But Plugin main.go registers `/manifest`.
        // So I need to fix Proxy or Plugin.
        // Plugin should perhaps listen on `/api/manifest` OR Proxy shouldn't force `/api` prefix on target.
        // "User Spec: /api/plugins/:pluginId/* -> base_url/api/*"

        // I will update the Plugin Main to move `/manifest` to `/api/manifest` to match the Proxy expectation.

        const res = await fetch(`/api/environments/${envId}/plugins/${pluginId}/manifest`);
        if (res.ok) {
            const data = await res.json();
            const manifest = data.body || data;
            pluginsStore.manifests[pluginId] = manifest;
        }
    } catch (e) {
        console.error(`Failed to load manifest for ${pluginId}`, e);
    }
}

export const pluginsStore = createPluginsStore();
