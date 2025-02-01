import { ref, computed } from 'vue';
import { MabiDB } from '@/mabidb';
import { ActorManager } from '@/eventActor';

export const loadingCount = ref(0);
export const isLoading = computed(() => loadingCount.value > 0);

const defaultRegion = 'kr';

export const region = ref(defaultRegion);
export const lang = ref(defaultRegion);
export const regionList = ref([defaultRegion]);

export const db = computed(() => {
    const instance = new MabiDB(region.value, lang.value);

    // // fire and forget
    // instance.open().catch(e => console.error(e));

    return instance;
});

export const raceNameMap = ref<Record<number, string>>({});
export const skillNameMap = ref<Record<number, string>>({});
export const condNameMap = ref<Record<number, string>>({});

export const actorManager = ref(new ActorManager());
