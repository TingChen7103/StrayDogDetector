import { ref, Ref } from 'vue';

import { ResourceData, ResourceVersion } from '@/protos/resourcedata';

export const resUrl = ref(`http://localhost:${__api_port}/res/`);

let loadingCount: Ref<number>;

export function resVerCall(path: string, opt?: HttpCallOpt): Promise<ResourceVersion> {
    return httpCall<ResourceVersion>(`${resUrl.value}${path}`, opt);
}

export async function resDataCall(path: string, opt?: HttpCallOpt): Promise<ResourceData> {
    const buf = await httpCallRaw(`${resUrl.value}${path}`, opt);

    return ResourceData.fromBinary(new Uint8Array(buf));
}

export type HttpCallOpt = {
    disableLoading?: boolean;
    reload?: boolean;
};

async function httpCall<T>(url: string, opt?: HttpCallOpt): Promise<T> {
    await setLoadingCount();

    try {
        const buf = await httpCallRaw(url, opt);
        const text = new TextDecoder('utf-8').decode(buf);
        return JSON.parse(text);
    }
    finally {
        if (!opt?.disableLoading) {
            loadingCount.value--;
        }
    }
}

async function httpCallRaw(url: string, opt?: HttpCallOpt): Promise<ArrayBuffer> {
    await setLoadingCount();

    try {
        if (!opt?.disableLoading) {
            loadingCount.value++;
        }

        const r = await fetch(url, {
            cache: opt?.reload ? 'reload' : undefined,
        });
        const buf = await r.arrayBuffer();
        if (r.status != 200) {
            throw new Error(`${r.status} ${new TextDecoder('utf-8').decode(buf)}`);
        }

        return buf;
    }
    finally {
        if (!opt?.disableLoading) {
            loadingCount.value--;
        }
    }
}

// 순환참조 제거용
async function setLoadingCount() {
    if (loadingCount) {
        return;
    }

    const { loadingCount: _loadingCount } = await import('@/store');
    loadingCount = _loadingCount;
}

export async function init(): Promise<void> {
    return;
}