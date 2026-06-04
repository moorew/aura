import { isTauri } from '$lib/tauri/bridge';

function createPlatformStore() {
    let desktop = $state(false);

    function init() {
        desktop = isTauri();
    }

    return {
        get desktop() { return desktop; },
        init,
    };
}

export const platform = createPlatformStore();
