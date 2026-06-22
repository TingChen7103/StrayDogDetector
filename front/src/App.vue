<template>
    <v-sheet width="100vw" class="d-flex flex-wrap pl-1 pr-1">
        <v-sheet width="100svw" class="d-flex align-center">
            <span style="text-wrap-mode: nowrap;">dilmatulgi, api
                <span v-if="socketConnected"><v-icon icon="mdi-check" color="success" />connected</span>
                <span v-else><v-icon icon="mdi-close" color="error" />disconnected</span>
            </span>
            <v-divider class="mx-2" vertical />
            <span style="text-wrap-mode: nowrap;">Mabinogi
                <span v-if="mabiConnected" class="text-success"><v-icon icon="mdi-check" color="success" />connected</span>
                <span v-else class="text-error"><v-icon icon="mdi-close" color="error" />disconnected</span>
            </span>
            <v-btn @click="connectMabi" :loading="isConnectingMabi" color="primary" size="x-small" variant="outlined" class="ml-2">Connect</v-btn>
            <v-divider class="mx-2" vertical /> <v-select
                v-model="lang"
                :items="langList"
                label="Lang"
                density="compact"
                hide-details
                class="mr-2" 
                style="max-width: 100px;"
            ></v-select>

            <v-tooltip>
                <template v-slot:activator="{ props }">
                    <v-btn @click="loadFromFile" v-bind="props" :loading="isLoading" color="primary" size="small"
                        prepend-icon="mdi-upload" class="ml-1">Load</v-btn>
                </template>
                파일에서 데이터를 로드합니다
            </v-tooltip>
            <v-btn @click="download" :loading="isLoading" color="primary" size="small" prepend-icon="mdi-download"
                class="ml-1">Download</v-btn>
            <v-tooltip>
                <template v-slot:activator="{ props }">
                    <v-btn @click="loadFromServer" v-bind="props" :loading="isLoading" color="primary" size="small"
                        prepend-icon="mdi-refresh" class="ml-1">Reload</v-btn>
                </template>
                서버에서 데이터를 다시 로드합니다
            </v-tooltip>
            <v-btn @click="clearData" :loading="isLoading" color="primary" size="small" prepend-icon="mdi-close"
                class="ml-1 mr-4">Clear</v-btn></v-sheet>
    </v-sheet>

    <v-tabs v-model="tab">
        <v-tab value="takeDamage">Take Damage</v-tab>
        <v-tab value="applyDamageByEntity">Apply Damage (By Entity)</v-tab>
        <v-tab value="applyDamageBySkill">Apply Damage (By Skill)</v-tab>
        <v-tab value="entityList">Characters</v-tab>
    </v-tabs>

    <v-tabs-window v-model="tab">
        <v-tabs-window-item value="takeDamage">
            <take-damage />
        </v-tabs-window-item>

        <v-tabs-window-item value="applyDamageByEntity">
            <apply-damage-by-entity />
        </v-tabs-window-item>

        <v-tabs-window-item value="applyDamageBySkill">
            <apply-damage-by-skill />
        </v-tabs-window-item>

        <v-tabs-window-item value="entityList">
            <entity-list />
        </v-tabs-window-item>
    </v-tabs-window>
</template>

<script lang="ts">
import { defineComponent, onMounted, inject, ref, watch } from "vue"; // 1. 加入 watch
import { SocketClient } from '@/socketClient';
import { eventBase } from "./protocols";

// Component imports...
import TakeDamageComponent from '@/components/takeDamage.vue';
import ApplyDamageByEntityComponent from '@/components/applyDamageByEntity.vue';
import ApplyDamageBySkillComponent from '@/components/applyDamageBySkill.vue';
import EntityListComponent from "./components/entityList.vue";

export default defineComponent({
    name: "App",
    components: {
        TakeDamage: TakeDamageComponent,
        ApplyDamageByEntity: ApplyDamageByEntityComponent,
        ApplyDamageBySkill: ApplyDamageBySkillComponent,
        EntityList: EntityListComponent,
    },
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const lang = inject('lang');
        const regionList = inject('regionList');
        const langList = inject('langList');
        const db = inject('db');
        const raceNameMap = inject('raceNameMap');
        const skillNameMap = inject('skillNameMap');
        const condNameMap = inject('condNameMap');
        const itemNameMap = inject('itemNameMap');
        const appEvent = inject('appEvent');
        const actorManager = inject('actorManager');
        const dcManager = inject('dcManager');

        const socketConnected = ref(false);
        const socket = new SocketClient(`/ws`);
        socket.onConnect = isConnected => socketConnected.value = isConnected;
        socket.onEvent = (event) => {
            // Discard incoming WS events to prevent automatic live updates
        };

        const mabiConnected = ref(false);
        const isConnectingMabi = ref(false);

        const checkMabiStatus = async () => {
            try {
                const res = await fetch('/api/mabinogi/status');
                if (res.ok) {
                    const data = await res.json();
                    mabiConnected.value = data.connected;
                }
            } catch (e) {
                console.error(e);
            }
        };

        const connectMabi = async () => {
            isConnectingMabi.value = true;
            try {
                const res = await fetch('/api/mabinogi/connect', { method: 'POST' });
                if (res.ok) {
                    const data = await res.json();
                    mabiConnected.value = data.connected;
                    if (data.connected) {
                        alert("瑪奇連線成功！");
                    } else {
                        alert("未偵測到瑪奇軟體，請確認遊戲已開啟並開始活動。");
                    }
                } else {
                    alert("連線請求失敗");
                }
            } catch (e) {
                console.error(e);
                alert("連線時發生錯誤: " + e);
            } finally {
                isConnectingMabi.value = false;
            }
        };

        const loadJsonData = (jsonStr: string) => {
            let lastPos = 0;
            let count = 0;

            while (lastPos < jsonStr.length) {
                const nextPos = jsonStr.indexOf('\n', lastPos);
                if (nextPos < 0) {
                    break;
                }

                const line = jsonStr.substring(lastPos, nextPos).trim();
                lastPos = nextPos + 1;
                count++;

                if (!line) {
                    continue;
                }

                try {
                    const event = JSON.parse(line);
                    actorManager.value.onEvent(event);
                }
                catch (e) {
                    console.error(e);
                    continue;
                }
            }

            console.log(`loaded ${count} events`);
        }

        const clearData = () => {
            appEvent.value.dispatchEvent(new CustomEvent('clear'));

            // clear했을 때 서버도 같이 clear하는게 맞을지?
            actorManager.value.clear();
            dcManager.value.clear();
        }

        const download = () => {
            window.open('/api/packet_log', '_blank');
        }

        const loadFromFile = async () => {
            const input = document.createElement('input');

            try {
                input.type = 'file';
                input.accept = '.ndjson';
                input.click();

                await new Promise<void>(resolve => {
                    input.addEventListener('cancel', () => {
                        console.log('file select cancel');
                        resolve();
                    });
                    input.addEventListener('change', () => {
                        console.log('file selected');
                        resolve();
                    });
                })

                if (!input.files?.length) {
                    // ?
                    return;
                }

                const file = input.files[0];
                const r = new FileReader();

                // 파일이 커지면 chunk로 읽는 것도 고려해야함
                r.readAsText(file);


                await new Promise<void>(resolve => {
                    r.addEventListener('abort', () => {
                        console.log('file read abort');
                        resolve();
                    });
                    r.addEventListener('error', () => {
                        console.log('file read error', r.error);
                        resolve();
                    });
                    r.addEventListener('load', () => {
                        console.log('file read complete');
                        resolve();
                    });
                });

                const jsonData = r.result as string || '';

                clearData();
                loadJsonData(jsonData);
            }
            finally {
                input.remove();
            }
        }

        const loadFromServer = async () => {
            try {
                const res = await fetch('/api/packet_log');
                if (!res.ok) {
                    throw new Error(`failed to fetch data ${res.status}`);
                }

                const jsonData = await res.text();

                clearData();
                loadJsonData(jsonData);
            }
            catch (e) {
                console.error(e);
                alert(`failed to load data ${e}`);
            }
        }

        const tab = ref('');

        const safeGetLang = (nameKey: string) => {
            try {
                if (!db.value) return nameKey;
                const str = db.value.getCurLangString(nameKey);
                return str ? str : nameKey;
            } catch (e) {
                return nameKey;
            }
        };

        const reloadDbData = async () => {
            if (!db.value) return;

            // 當 Store 的 lang 改變，db 會是新的實例，必須重新 tryOpen
            // tryOpen 內部會根據 lang 去抓對應的翻譯檔
            await db.value.tryOpen();
            
            console.log(`[App] Reloading Data... Region: ${region.value}, Lang: ${lang.value}`);

            // 讀取 Race
            {
                const list = await db.value.getSortedListData('RaceList');
                for (const v of list) {
                    raceNameMap.value[v.Id] = `${safeGetLang(v.Name)} ${v.Id}`;
                }
            }
            // 讀取 Skill (最容易爆的地方)
            {
                const list = await db.value.getSortedListData('SkillList');
                for (const v of list) {
                    skillNameMap.value[v.Id] = safeGetLang(v.Name) || `Skill_${v.Id}`;
                }
            }
            // 讀取 Conditions
            {
                const list = await db.value.getSortedListData('CharCondList');
                for (const v of list) {
                    condNameMap.value[v.Id] = `${safeGetLang(v.Name)} ${v.Id}`;
                }
            }
            // 讀取 Items
            {
                const list = await db.value.getSortedListData('ItemList');
                for (const v of list) {
                    itemNameMap.value[v.Id] = `${safeGetLang(v.Name)} ${v.Id}`;
                }
            }
        };

        watch(db, async () => {
            await reloadDbData();
        });

        onMounted(async () => {
            // 初始化選單列表
            regionList.value = ['kr', 'krt', 'cn', 'jp', 'tw', 'us'];
            langList.value = ['kr', 'krt', 'cn', 'jp', 'tw', 'us'];

            // 初始載入
            await reloadDbData();
            await loadFromServer();

            socket.connect();

            await checkMabiStatus();
            setInterval(checkMabiStatus, 5000);
        });

        return {
            isLoading,
            region,
            lang,
            regionList,
            langList,
            socketConnected,
            clearData,
            download,
            loadFromFile,
            loadFromServer,

            tab,
            mabiConnected,
            isConnectingMabi,
            connectMabi,
        }
    }
});

</script>
