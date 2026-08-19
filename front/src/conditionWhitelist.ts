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
