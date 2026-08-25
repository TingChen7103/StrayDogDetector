/**
 * Buff 覆蓋率白名單
 *
 * 判讀一律以 CCId 為準,不以名稱比對 —— 因為條件名稱會隨遊戲改版在地化
 * (例如 1174 目前仍是韓文「퓨리어스 앱솔루트」,日後可能被翻成中文),
 * 用名稱當判準會在改版後失效。
 *
 * 唯一的例外是「魔法陣」系列: 它是一整類同性質條件、且會隨版本新增,
 * 因此改用名稱規則整類納入 (見 buffWhitelistNamePatterns)。
 */

/** 明確指定納入 Buff 覆蓋率的 CCId */
export const buffWhitelistIds = new Set<number>([
    62,   // 魔法施展速度增加
    63,   // 攻擊力增加
    192,  // 活潑板
    193,  // 進行曲
    471,  // 娃娃同行獎勵
    476,  // 要害貫通
    511,  // 銳利
    512,  // 再生
    513,  // 迅速
    515,  // 逆轉
    577,  // 精靈實體化技能增益效果
    628,  // 魔法施展速度增加
    680,  // 戰場的序曲
    914,  // 肉球的庇護
    915,  // 肉球的庇護
    938,  // 魔力穿刺
    978,  // 處決
    1023, // 月亮
    1033, // 星星
    1035, // 太陽
    1036, // 月亮
    1037, // 日蝕
    1086, // 要害貫通強化
    1121, // 魔法攻擊力強化
    1123, // 鍊合轉換
    1145, // 觸媒效應
    1159, // 狂怒意志狀態
    1174, // 퓨리어스 앱솔루트 (尚未在地化)
    1225, // 種子燃燒
]);

/**
 * 以名稱納入的規則: 任何「◯◯魔法陣」都算。
 * 名稱取自 condNameMap,格式為 `<名稱> <CCId>`。
 */
export const buffWhitelistNamePatterns: RegExp[] = [
    /魔法陣/,
];

/**
 * 名稱尚未載入時的備援: 魔法陣的 CCId 全部落在 10000 以上。
 * (實測資源檔中 CCId >= 10000 的 131 個條件 100% 都是魔法陣,無一例外;
 *  最小 10000、最大 10138。) 只有在取不到名稱時才會用到這條規則,
 * 名稱正常時仍以 buffWhitelistNamePatterns 為準。
 */
export const magicCircleIdFloor = 10000;

/**
 * 狀態支援: 銳利 / 再生 / 迅速 / 逆轉 幾乎總是成組出現,
 * 同時出現 statusSupportMergeThreshold 個以上時合併為單一列,
 * 避免佔掉四格顯示空間。
 */
export const statusSupportIds = [511, 512, 513, 515];
/** 幾個以上才合併 (3 或 4 個時合併,1~2 個時維持原樣分開顯示) */
export const statusSupportMergeThreshold = 3;
/** 合併後沿用此 CCId,圖示即為「515 逆轉」的圖示 */
export const statusSupportDisplayId = 515;
export const statusSupportDisplayName = '狀態支援';

/**
 * 判斷某個條件是否要顯示在 Buff 覆蓋率中。
 * @param ccId 條件 id (主要判準)
 * @param condName 條件名稱,僅供「魔法陣」這類名稱規則使用;取不到時傳 undefined 即可
 */
export function isWhitelistedBuff(ccId: number, condName?: string): boolean {
    if (buffWhitelistIds.has(ccId)) {
        return true;
    }

    if (condName) {
        for (const pattern of buffWhitelistNamePatterns) {
            if (pattern.test(condName)) {
                return true;
            }
        }

        return false;
    }

    // 名稱還沒載入 (資源檔載入中或抓取失敗) 時,改用 id 範圍判斷魔法陣,
    // 避免整類魔法陣在名稱可用之前被誤篩掉
    return ccId >= magicCircleIdFloor;
}

/* ============================ Debuff ============================ */

/**
 * Debuff 覆蓋率白名單。
 *
 * 陣列順序 = 畫面上由左至右的顯示順序。
 * 每個項目本身是一組 CCId:長度 1 表示單一條件;長度 >1 表示
 * 「遊戲設定中本質相同」的條件,覆蓋率會合併計算 (任一個在身上就算),
 * 並以組內第一個 id 作為代表 (icon 與名稱都取代表 id)。
 */
export const debuffWhitelistGroups: number[][] = [
    [1164],        // 減少防禦/保護
    [1165],        // 減少魔法防禦/魔法保護
    [1166],        // 所受傷害增加
    [1093],        // 保護最大減少
    [1094],        // 魔法保護最大減少
    [912, 913],    // 喵喵的喵皇降臨 (兩者本質相同,合併)
    [504, 392],    // 雷電交加 / 纏繞的閃電 (兩者本質相同,合併)
    [1138],        // 幸運草標記
    [598],         // 跑跑卡丁車水球
    [1026],        // 倒吊人
    [803],         // 崩壞的波動
    [10001],       // 物理防禦和保護減少瑪奇魔法陣
    [10002],       // 魔法防禦和保護減少瑪奇魔法陣
    [464],         // 冰雪狀態
    [1004],        // 銳利目光
];

/** 白名單內所有 CCId (含合併組的成員) */
export const debuffWhitelistIds = new Set<number>(debuffWhitelistGroups.flat());

/** CCId -> 代表 id (合併組的成員都對應到組內第一個 id;非合併組即自己) */
const debuffRepresentativeMap = new Map<number, number>(
    debuffWhitelistGroups.flatMap(group => group.map(id => [id, group[0]] as [number, number]))
);

/** 取得該條件在顯示上的代表 id;不在白名單內則回傳自己 */
export function debuffRepresentativeId(ccId: number): number {
    return debuffRepresentativeMap.get(ccId) ?? ccId;
}

/** 白名單排序用的名次;不在白名單內回傳 -1 */
export function debuffWhitelistRank(ccId: number): number {
    return debuffWhitelistGroups.findIndex(group => group.includes(ccId));
}

export function isWhitelistedDebuff(ccId: number): boolean {
    return debuffWhitelistIds.has(ccId);
}

export type DebuffCoverageEntry = {
    ccId: number;
    percentage: number;
};

/** computeDebuffCoverage 只需要 TargetConditions,不綁定完整的 EntityDamage 型別 */
type DamageWithTargetConditions = {
    TargetConditions?: { CCId: number }[];
};

/**
 * 計算一組傷害事件對應的 Debuff 覆蓋率。
 *
 * 基準與 Buff 覆蓋率一致: 這些傷害事件中,有幾成發生的當下該 debuff 掛在目標身上。
 * 合併組 (912/913、504/392) 以聯集計算 —— 任一個在身上就算,並歸到代表 id。
 *
 * 白名單的每一格一定會出現,對該敵人從未用過的就顯示 0%,
 * 讓使用者一眼看出自己缺了哪個 debuff。
 *
 * @param showAll false = 只算白名單;true = 全部條件都算
 * @returns 白名單項目依白名單順序排在前面,其餘依覆蓋率由高到低接在後面
 */
export function computeDebuffCoverage(
    damages: DamageWithTargetConditions[],
    showAll: boolean,
): DebuffCoverageEntry[] {
    if (damages.length === 0) {
        return [];
    }

    const total = damages.length;
    const countMap: Record<number, number> = {};

    // 白名單每一格都先預設為 0,沒用過的 debuff 才會顯示成 0% 而不是整格消失
    for (const group of debuffWhitelistGroups) {
        countMap[group[0]] = 0;
    }

    for (const d of damages) {
        if (!d.TargetConditions) {
            continue;
        }

        // 先映射到代表 id,合併組因此自然成為聯集計數
        const seen = new Set<number>();
        for (const c of d.TargetConditions) {
            if (!showAll && !isWhitelistedDebuff(c.CCId)) {
                continue;
            }
            seen.add(debuffRepresentativeId(c.CCId));
        }

        for (const ccId of seen) {
            countMap[ccId] = (countMap[ccId] || 0) + 1;
        }
    }

    return Object.entries(countMap)
        .map(([ccIdStr, count]) => ({
            ccId: +ccIdStr,
            percentage: (count / total) * 100,
        }))
        .sort((a, b) => {
            const ra = debuffWhitelistRank(a.ccId);
            const rb = debuffWhitelistRank(b.ccId);

            // 白名單一律排在前面,並依白名單既定順序
            if (ra >= 0 && rb >= 0) return ra - rb;
            if (ra >= 0) return -1;
            if (rb >= 0) return 1;

            // 白名單外的依覆蓋率由高到低
            return b.percentage - a.percentage;
        });
}

/* ======================= 音樂 buff 效果數值 ======================= */

/**
 * 音樂 buff 的實際加成數值顯示設定。
 *
 * 數值來源是條件封包的參數字串 (格式 `鍵:型別:值;`),由後端帶進
 * eventCharacterConditionEnable.Params。後端只對
 * eventPublisher.go 的 conditionParamCCIds 內的條件輸出參數,
 * 要新增條件時兩邊都要加。
 *
 * kind:
 *   percent    — 欄位值本身就是百分比 (66.363777 -> 66.36%)
 *   multiplier — 欄位值是倍率,顯示值 = (值 - 1) x 100% (1.356952 -> 35.70%)
 *
 * 對照方式: data_source/音樂對照用.pcapng 逐值比對遊戲畫面,四捨五入後完全相同。
 */
export type MusicBuffField = {
    key: string;
    label: string;
    kind: 'percent' | 'multiplier';
};

export const musicBuffDisplay: Record<number, MusicBuffField[]> = {
    680: [
        // 最小攻擊力(MCMBAMIN)實測恆等於最大,只顯示最大
        { key: 'MCMBAMAX', label: '最大攻擊力', kind: 'percent' },
    ],
    192: [
        // LSMA 同時是「魔法攻擊力」與「鍊金術傷害」(封包只有一個欄位),合併顯示
        { key: 'LSMA', label: '魔法/煉金傷', kind: 'percent' },
        { key: 'MFCP', label: '施展魔法速度', kind: 'percent' },
    ],
    193: [
        { key: 'SPDPC', label: '移動速度', kind: 'multiplier' },
    ],
};

export function hasMusicBuffDisplay(ccId: number): boolean {
    return musicBuffDisplay[ccId] !== undefined;
}

/** 解析 `鍵:型別:值;` 參數字串,只取 float 欄位 */
export function parseConditionParams(params: string): Record<string, number> {
    const out: Record<string, number> = {};

    for (const part of params.split(';')) {
        const seg = part.split(':');
        if (seg.length !== 3 || seg[1] !== 'f') {
            continue;
        }

        const v = parseFloat(seg[2]);
        if (!isNaN(v)) {
            out[seg[0]] = v;
        }
    }

    return out;
}

function formatFieldValue(field: MusicBuffField, raw: number): string {
    const pct = field.kind === 'multiplier' ? (raw - 1) * 100 : raw;
    return `${field.label} ${pct.toFixed(2)}%`;
}

/** 把一次施放的參數字串轉成可讀文字,例如「最大攻擊力 66.36%」 */
export function formatMusicBuffParams(ccId: number, params: string | undefined): string {
    const fields = musicBuffDisplay[ccId];
    if (!fields || !params) {
        return '';
    }

    const kv = parseConditionParams(params);

    return fields
        .filter(f => kv[f.key] !== undefined)
        .map(f => formatFieldValue(f, kv[f.key]))
        .join('　');
}

export type MusicBuffBreakdownEntry = {
    /** 該次施放的加成數值文字 */
    text: string;
    /** 佔「此 buff 生效期間」的比例,同一個 buff 的所有項目相加為 100% */
    percentage: number;
};

/** computeMusicBuffBreakdown 只需要 Conditions */
type DamageWithConditions = {
    Conditions?: { CCId: number; Params?: string }[];
};

/**
 * 統計某個音樂 buff 在生效期間內,各種加成數值各佔多少。
 *
 * 分母是「該 buff 有生效的傷害事件數」而非全部傷害 ——
 * 因此回傳的百分比相加為 100%,語意是
 * 「在你有這個 buff 的時間裡,吃到的是哪個版本」。
 * (外層圖示上的覆蓋率分母才是全部傷害事件,兩者意義不同。)
 *
 * 詩人中途重補且數值不同時,這裡就會出現多筆。
 */
export function computeMusicBuffBreakdown(
    damages: DamageWithConditions[],
    ccId: number,
): MusicBuffBreakdownEntry[] {
    if (!hasMusicBuffDisplay(ccId)) {
        return [];
    }

    let activeCount = 0;
    const counts: Record<string, number> = {};

    for (const d of damages) {
        const cond = d.Conditions?.find(c => c.CCId === ccId);
        if (!cond) {
            continue;
        }

        activeCount++;

        const text = formatMusicBuffParams(ccId, cond.Params) || '(無數值資料)';
        counts[text] = (counts[text] || 0) + 1;
    }

    if (activeCount === 0) {
        return [];
    }

    return Object.entries(counts)
        .map(([text, count]) => ({
            text,
            percentage: (count / activeCount) * 100,
        }))
        .sort((a, b) => b.percentage - a.percentage);
}
