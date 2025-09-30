# 災害物資需求後端 (Go + PostgreSQL)

本專案依照需求圖片實作：

功能：
1. 建立物資需求 (POST /requests)
2. 取得需求清單 (GET /requests) － 已支援分頁 (limit / offset) 與 JSON-LD Collection 包裝
3. 物資配送登記 (POST /supplies/distribute)
4. 物資扁平清單 (GET /supplies) － 支援依 request_id 過濾 + 分頁
5. OpenAPI 規格檔 `openapi.yaml`

擴充欄位 (圖片下方列出的 API 欄位) 亦已納入：
- code 站點名稱/代碼
- status 狀態 (pending / partial / fulfilled / closed)
- needed_people 所需人數
- contact 聯繫資訊 (若僅有 phone 會自動帶入)
- notes 備註
- lng, lat 經緯度
- map_link 地圖或導航連結
- created_at Unix timestamp (秒)

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

可自行 `export` 或用 shell 載入：
```
set -a; source .env; set +a
```

## 啟動
```
go build ./...
go run ./cmd/server
```

啟動時會自動執行簡易 migration，建立資料表 (若不存在)。

## API 總覽
詳見 `openapi.yaml`。重點：

### 分頁與 JSON-LD Collection 格式
目前 `GET /requests` 與 `GET /supplies` 回傳格式採用簡化 Hydra / JSON-LD Collection：

```jsonc
{
  "@context": "https://www.w3.org/ns/hydra/context.jsonld",
  "@type": "Collection",
  "totalItems": 123,          // 總筆數
  "member": [ /* 陣列資料 */ ],
  "limit": 20,                // 本頁大小
  "offset": 40,               // 從第幾筆開始 (0-based)
  "next": "/requests?limit=20&offset=60",    // 若無後續頁面為 null
  "previous": "/requests?limit=20&offset=20" // 若無前一頁為 null
}
```

Query 參數：
- limit: 每頁筆數 (requests: 預設 20, 最大 200；supplies: 預設 50, 最大 500)
- offset: 起始偏移 (0 為第一頁)
- status: (僅 /requests) 過濾需求狀態
- request_id: (僅 /supplies) 過濾特定需求下物資

範例：
```
GET /requests?limit=20&offset=0
GET /requests?status=pending&limit=10&offset=30
GET /supplies?request_id=<uuid>&limit=100
```

### 建立需求
POST /requests
```json
{
  "code":"GF001",
  "name":"需要救援單位 A",
  "address":"地址",
  "phone":"0912...",
  "contact":"聯絡人資訊，可含電話",
  "status":"pending",
  "needed_people":30,
  "notes":"備註",
  "lng":121.5,
  "lat":25.0,
  "map_link":"https://maps.example/...",
  "supplies": [
    {"tag":"food","name":"罐頭","total_count":100,"unit":"箱"},
    {"tag":"medical","name":"繃帶","total_count":200,"unit":"包"}
  ]
}
```

也可傳單一物資物件 (符合圖片原格式)：
```json
{
  "name":"單位名稱",
  "address":"...",
  "phone":"...",
  "supplies": {"tag":"food","name":"餅乾","total_count":50,"unit":"箱"}
}
```

### 取得需求清單
GET /requests

回傳包含每個需求及其 supplies，`created_at` 為 Unix 秒，外層使用 JSON-LD Collection 包裝 (見上)。

可用 `status` query 過濾：`/requests?status=pending`。

### 物資清單 (扁平)
GET /supplies

回傳所有物資 (不含需求主檔欄位)，支援：
```
GET /supplies?limit=50&offset=0
GET /supplies?request_id=<需求UUID>&limit=100
```
回傳同樣套用 JSON-LD Collection。

### 物資配送
POST /supplies/distribute
```json
[
  {"id":"<supply-item-uuid>", "count":10},
  {"id":"<另一個>", "count":5}
]
```
會將 `received_count` 累加；若超過 `total_count` 會回 400。

## OpenAPI
`openapi.yaml` 可匯入 Swagger UI / Redoc。


---
有需要再調整欄位或增加端點，歡迎提出！