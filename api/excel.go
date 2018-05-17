package api

type ExcelRow struct {
	ItemNO    string  `json:"item_no" xlsx:"0"`
	OfflineID string  `json:"offline_id" xlsx:"1"`
	ShopName  string  `json:"shop_name" xlsx:"2"`
	Quantity  float64 `json:"quantity" xlsx:"3"`
	Price     float64 `json:"price" xlsx:"4"`
}
