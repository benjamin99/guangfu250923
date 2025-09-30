# 災害物資 / 支援資源後端 (Go + PostgreSQL)

以快速支援災後資訊蒐集、物資供應追蹤、人力/設施資源盤點為目標的 API。採用 Go + Gin + PostgreSQL，提供 JSON-LD 風格的分頁集合回應格式。

## 主要功能模組
| 模組 | Path 前綴 | 說明 |
|------|-----------|------|
| 供應單 / 物資 | `/supplies`, `/supply_items` | 建立供應單、單筆/多筆物資項目管理、批次配送 (累加 `recieved_count`) |
| 志工招募單位 | `/volunteer_organizations` | 招募或協作單位資訊 |
| 庇護所 | `/shelters` | 庇護 / 收容點資訊 |
| 醫療站 | `/medical_stations` | 醫療支援站點 |
| 心理健康資源 | `/mental_health_resources` | 諮商 / 心理支持資源 |
| 住宿資源 | `/accommodations` | 臨時住宿或安置資訊 |
| 沐浴/淋浴 | `/shower_stations` | 行動浴室 / 盥洗點 |
| 飲水補給 | `/water_refill_stations` | 飲水補給點 |
| 廁所 | `/restrooms` | 臨時 / 既有廁所點 |
| 人力需求 | `/human_resources` | 人力角色與填補狀態 |
| 要求紀錄 | `/_admin/request_logs` | 最近 API 請求 (管理用途) |
| Sheet 快取 | `/sheet/snapshot` | 從 Google Sheet 載入的快取快照 |
| 健康檢查 | `/healthz` | 基本健康檢查 |

完整欄位與 Schema 參考 `openapi.yaml`。

## JSON-LD 分頁格式
所有「集合型」(list) GET 端點採統一格式：

```jsonc
{
  "@context": "https://www.w3.org/ns/hydra/context.jsonld",
  "@type": "Collection",
  "totalItems": 123,
  "member": [ /* 資料陣列 */ ],
  "limit": 50,
  "offset": 0,
  "next": "/supplies?limit=50&offset=50",    // 無則為 null
  "previous": null
}
```

## 供應單 (Supply) 與物資項目 (SupplyItem)

設計重點：
- 建立供應單時可「可選」內嵌 1 個初始物資項目 (payload 的 `supplies` 為單一物件)。
- 供應單查詢 (`GET /supplies/{id}`) 回傳 `supplies` 為「陣列」(可能為空)。
- 物資項目欄位採用 `recieved_count`（刻意沿用前端既有錯字，不是 received）。
- 數量邏輯：`recieved_count <= total_count`，批次配送與 PATCH 都會驗證。

### 建立供應單
POST `/supplies`
```json
{
  "name": "光復倉庫 A",
  "address": "光復鄉中正路 123 號",
  "phone": "03-8700000",
  "notes": "民間協助據點",
  "supplies": {
    "tag": "food",
    "name": "白米",
    "total_count": 500,
    "unit": "公斤",
    "recieved_count": 50
  }
}
```
回應 (201)：
```json
{
  "@context": "https://www.w3.org/ns/hydra/context.jsonld",
  "@type": "Supply",
  "id": "<uuid>",
  "name": "光復倉庫 A",
  "address": "光復鄉中正路 123 號",
  "created_at": 1728000000,
  "updated_at": 1728000000,
  "supplies": [
  {"id":"<item-uuid>","supply_id":"<uuid>","tag":"food","name":"白米","recieved_count":50,"total_count":500,"unit":"公斤"}
  ]
}
```

### 取得單一供應單
GET `/supplies/{id}`

回應：同上格式；若無物資項目則 `"supplies": []`。

### 列出供應單
GET `/supplies?limit=50&offset=0`

目前列表中的每個 `member` 供應單物件（暫未嵌入 supplies 陣列）只含基本欄位；需要細項時請再呼叫 `GET /supplies/{id}`。未來若要內嵌第一筆或全部項目，可再調整。

### 建立物資項目
POST `/supply_items`
```json
{
  "supply_id": "<supply-uuid>",
  "tag": "medical",
  "name": "繃帶",
  "total_count": 200,
  "unit": "卷"
}
```
回應：`{ "id": "<item-uuid>" }`

### 更新物資項目 (部分欄位)
PATCH `/supply_items/{id}`
```json
{ "recieved_count": 120 }
```
若更新後 `recieved_count > total_count` 會回 400。

### 批次配送 (累加數量)
POST `/supplies/{id}`  （注意：不是舊版的 `/supplies/distribute`）
```json
[
  {"id": "<item-uuid-1>", "count": 10},
  {"id": "<item-uuid-2>", "count": 5}
]
```
成功：回傳更新後的物資項目陣列。

失敗範例 (超過 total_count)：
```json
{
  "error": "exceeds total_count",
  "id": "<item-uuid-1>",
  "recieved_count": 95,
  "total_count": 100,
  "attempt_add": 10
}
```

### 物資欄位摘要
| 欄位 | 說明 |
|------|------|
| tag | 分類 (food, medical, etc.) |
| name | 物資名稱 |
| recieved_count | 已取得 / 已配送數量 (錯字沿用) |
| total_count | 需求或目標數量 |
| unit | 單位 (箱, 包, 公斤, 人, 卷...) |

## 其他資源端點
其餘（庇護所 / 醫療站 / 心理健康 / 住宿 / 沐浴 / 飲水 / 廁所 / 志工招募 / 人力需求）皆採類似模式：
- POST 建立
- GET 列表（分頁 JSON-LD）
- GET {id} 單筆
- PATCH {id} 部分更新（僅部分資源支援）

## 錯誤格式
大多數錯誤：`{ "error": "<訊息>" }`
部分情境（批次配送）會附加額外欄位 (id, recieved_count, total_count, attempt_add)。

## 命名特別說明
- `recieved_count`：與前端既有欄位保持一致的錯字；資料庫內部欄位仍為 `received_count`。
- （重大變更）`suppily_items` 已更名為 `supply_items`，欄位 `suppily_id` 改為 `supply_id`，Schema/路徑/操作 ID 同步更新。

## 環境變數
從環境讀取 (未使用外部 dotenv 套件)，可參考 `.env.example`：

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=relief
DB_SSLMODE=disable
PORT=8080
```

載入方式：
```
set -a; source .env; set +a
```

## 執行
```
go build ./...
go run ./cmd/server
```
啟動時會執行簡易 migration（不存在才建立）。

## OpenAPI 規格
檔案：`openapi.yaml`（可直接以 `/openapi.yaml` 提供、Swagger UI: `/swagger/`）。

### Lint (Spectral)
專案含 CI 工作流程：
```
spectral lint --ruleset .spectral.yaml openapi.yaml
```

## 待改進 (Roadmap)
- ListSupplies 是否要內嵌部分或全部物資項目
- Error schema 標準化 (統一格式 + code)
- 欄位/Schema 測試與自動化驗證

---
需要新增欄位或端點，歡迎提出 Issue / PR。
