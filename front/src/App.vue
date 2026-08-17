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

            <v-checkbox
                v-model="saveRawData"
                label="Log Raw Data"
                density="compact"
                hide-details
                color="primary"
                class="mr-2"
            ></v-checkbox>

            <v-tooltip>
                <template v-slot:activator="{ props }">
                    <v-btn @click="loadFromFile" v-bind="props" :loading="isLoading" color="primary" size="small"
                        prepend-icon="mdi-upload" class="ml-1">Load</v-btn>
                </template>
                從本機檔案載入數據分析
            </v-tooltip>
            <v-btn @click="download" :loading="isLoading" color="primary" size="small" prepend-icon="mdi-download"
                class="ml-1">Download</v-btn>
            <v-tooltip>
                <template v-slot:activator="{ props }">
                    <v-btn @click="loadFromServer" v-bind="props" :loading="isLoading" color="primary" size="small"
                        prepend-icon="mdi-refresh" class="ml-1">Reload</v-btn>
                </template>
                重新載入伺服器數據
            </v-tooltip>
            <v-btn @click="clearData" :loading="isLoading" color="primary" size="small" prepend-icon="mdi-close"
                class="ml-1 mr-4">Clear</v-btn></v-sheet>
    </v-sheet>

    <v-tabs v-model="tab">
        <v-tab value="applyDamageBySkill">Apply Damage (By Skill)</v-tab>
        <v-tab value="entityList">Characters</v-tab>
    </v-tabs>

    <v-tabs-window v-model="tab">
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
        
        const saveRawData = ref(false);
        fetch('/api/settings').then(r => r.json()).then(data => {
            if (data.SaveRawData === "true") saveRawData.value = true;
        }).catch(() => {});

        watch(saveRawData, (newVal) => {
            fetch('/api/settings', {
                method: 'POST',
                body: JSON.stringify({ SaveRawData: newVal ? "true" : "false" })
            }).catch(console.error);
        });

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
                await fetch('/api/settings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ SaveRawData: saveRawData.value ? "true" : "false" })
                });
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

        const tab = ref('applyDamageBySkill');

        const localOverrides: Record<string, string> = {
            'race.13571': '雷楠的米勒：悔恨',
            'race.13570': '雷楠的米勒',
        };

        const safeGetLang = (nameKey: string) => {
            try {
                if (localOverrides[nameKey]) {
                    return localOverrides[nameKey];
                }
                if (!db.value) return nameKey;
                const str = db.value.getCurLangString(nameKey);
                return str ? str : nameKey;
            } catch (e) {
                return nameKey;
            }
        };

        const localSkillOverrides: Record<number, string> = {
            54201: '1. 幕後操作',
            54202: '2. 幕前傀儡',
            54151: '3. 第一幕 : 偶然的衝突',
            54152: '4. 第二幕 : 激增的憤怒',
            54153: '5. 第四幕 : 嫉妒的化身',
            54154: '6. 第六幕 : 誘惑的圈套',
            54155: '7. 第七幕 : 狂亂的奔走',
            59167: '8. 重拍強音',
            59168: '9. 律動步伐',
            59169: '10. 壯麗終曲',
            59164: '11. 弦音編織',
            59165: '12. 間奏斬擊',
            52503: '荊棘之光',
        };

        const reloadDbData = async () => {
            if (!db.value) return;

            // 當 Store 的 lang 改變，db 會是新的實例，必須重新 tryOpen
            // tryOpen 內部會根據 lang 去抓對應的翻譯檔
            await db.value.tryOpen();
            
            console.log(`[App] Reloading Data... Region: ${region.value}, Lang: ${lang.value}`);

            const hasLangDiff = region.value !== lang.value;

            // 讀取 Race
            {
                const list = await db.value.getSortedListData('RaceList', region.value);
                const langList = hasLangDiff ? await db.value.getSortedListData('RaceList', lang.value) : [];
                const langMap = new Map(langList.map(v => [v.Id, v.Name]));
                for (const v of list) {
                    const nameKey = langMap.get(v.Id) || v.Name;
                    raceNameMap.value[v.Id] = `${safeGetLang(nameKey)} ${v.Id}`;
                }
            }
            // 讀取 Skill (最容易爆的地方)
            {
                const list = await db.value.getSortedListData('SkillList', region.value);
                const langList = hasLangDiff ? await db.value.getSortedListData('SkillList', lang.value) : [];
                const langMap = new Map(langList.map(v => [v.Id, v.Name]));
                for (const v of list) {
                    const nameKey = langMap.get(v.Id) || v.Name;
                    skillNameMap.value[v.Id] = safeGetLang(nameKey) || `Skill_${v.Id}`;
                }
                for (const [sidStr, sname] of Object.entries(localSkillOverrides)) {
                    const sid = +sidStr;
                    if (!skillNameMap.value[sid] || skillNameMap.value[sid].startsWith('Skill_') || skillNameMap.value[sid].startsWith('skill.')) {
                        skillNameMap.value[sid] = sname;
                    }
                }
            }
            // 讀取 Conditions
            {
                const list = await db.value.getSortedListData('CharCondList', region.value);
                const langList = hasLangDiff ? await db.value.getSortedListData('CharCondList', lang.value) : [];
                const langMap = new Map(langList.map(v => [v.Id, v.Name]));
                for (const v of list) {
                    const nameKey = langMap.get(v.Id) || v.Name;
                    condNameMap.value[v.Id] = `${safeGetLang(nameKey)} ${v.Id}`;
                }
            }
            // 讀取 Items
            {
                const list = await db.value.getSortedListData('ItemList', region.value);
                const langList = hasLangDiff ? await db.value.getSortedListData('ItemList', lang.value) : [];
                const langMap = new Map(langList.map(v => [v.Id, v.Name]));
                for (const v of list) {
                    const nameKey = langMap.get(v.Id) || v.Name;
                    itemNameMap.value[v.Id] = `${safeGetLang(nameKey)} ${v.Id}`;
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
            saveRawData,

            tab,
            mabiConnected,
            isConnectingMabi,
            connectMabi,
        }
    }
});

</script>
