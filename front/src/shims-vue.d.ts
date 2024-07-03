import { ComputedRef, Ref } from 'vue';

import { MabiDB } from '@/mabidb';

declare module '@vue/runtime-core' {
    function inject(key: 'isLoading'): ComputedRef<boolean>;
    function inject(key: 'region'): Ref<string>;
    function inject(key: 'lang'): Ref<string>;
    function inject(key: 'regionList'): Ref<string[]>;
    function inject(key: 'db'): ComputedRef<MabiDB>;
}