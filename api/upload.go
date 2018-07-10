package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/tealeg/xlsx"
	"github.com/xuyuntech/inventory_sync_go/youzan"
)

type goodUpdatedSku struct {
	K          string  `json:"k"`
	V          string  `json:"v"`
	Price      float64 `json:"price"`
	Quantity   float64 `json:"quantity"`
	ToPrice    float64 `json:"to_price"`
	ToQuantity float64 `json:"to_quantity"`
	ID         int64   `json:"id"`
}
type goodUpdated struct {
	OfflineID   int64             `json:"offline_id"`
	OfflineName string            `json:"offline_name"`
	ItemID      int64             `json:"item_id"`
	ItemTitle   string            `json:"item_title"`
	ItemNo      string            `json:"item_no"`
	Skus        []*goodUpdatedSku `json:"skus"`
}

func (a *Api) Upload(c *gin.Context) {
	var (
		err        error
		file       *multipart.FileHeader
		fileReader multipart.File
		excelFile  *xlsx.File
		bs         []byte
		itemsHash  map[string][]*ExcelRow
	)
	file, err = c.FormFile("file")
	if err != nil {
		RespErr(c, err)
		return
	}
	fileReader, err = file.Open()
	if err != nil {
		RespErr(c, err)
		return
	}
	bs, err = ioutil.ReadAll(fileReader)
	if err != nil {
		RespErr(c, err)
		return
	}
	excelFile, err = xlsx.OpenBinary(bs)
	if err != nil {
		RespErr(c, err)
		return
	}
	if len(excelFile.Sheets) != 1 {
		RespErr(c, errors.New("excel 文件 sheet 大于 1, 请确保上传的是库存文件"))
		return
	}
	itemsHash = make(map[string][]*ExcelRow)
	sheet := excelFile.Sheets[0]
	for _, row := range sheet.Rows[1:] {
		excelRow := &ExcelRow{}
		if err := row.ReadStruct(excelRow); err != nil {
			vals := make([]string, 0)
			for _, cell := range row.Cells {
				vals = append(vals, cell.Value)
			}
			logrus.Errorf("parse excel row err: (%v), vals: %+v", err, vals)
		}
		if _, ok := itemsHash[excelRow.ItemNO]; !ok {
			itemsHash[excelRow.ItemNO] = make([]*ExcelRow, 0)
		}
		itemsHash[excelRow.ItemNO] = append(itemsHash[excelRow.ItemNO], excelRow)
	}
	InventorySyncTaskCache = &InventorySyncTask{
		Status:    "init",
		ItemsHash: itemsHash,
	}
	Resp(c, nil)

}

type goodsDetailResponse struct {
	Response struct {
		Item *youzan.GoodsDetail `json:"item"`
	} `json:"response"`
}

func parseGoodsDetail(b []byte) *youzan.GoodsDetail {
	gdr := &goodsDetailResponse{}
	if err := json.Unmarshal(b, gdr); err != nil {
		logrus.Errorf("转换 GoodsDetail 出错(%v), b: (%s)", err, string(b))
		return nil
	}
	// 格式化 sku
	detail := gdr.Response.Item
	if detail == nil {
		return nil
	}
	var err error
	for _, sku := range detail.Skus {
		sku.Price, err = strconv.ParseFloat(sku.PriceStr, 64)
		if err != nil {
			logrus.Errorf("商品 [%s] sku 价格转换错误: %f", sku.Price)
			continue
		}
		sku.Quantity, err = strconv.ParseFloat(sku.QuantityStr, 64)
		if err != nil {
			logrus.Errorf("商品 [%s] sku 库存转换错误: %f", sku.Quantity)
			continue
		}
	}
	return detail
}

func needYouzanGoodToBeUpdated(excelRow *ExcelRow, gd *youzan.GoodsDetail) (bool, error) {
	if len(gd.Skus) <= 0 {
		return false, fmt.Errorf("有赞商品 [%s] 没有设置 sku", gd.Title)
	}
	sku := gd.Skus[0]
	if sku.OuterID != gd.OuterID {
		return false, fmt.Errorf("有赞商品 [%s] 第一个 sku 的商品编码设置不正确", gd.Title)
	}

	if sku.Price != excelRow.Price || sku.Quantity != excelRow.Quantity {
		return true, nil
	}
	return false, nil
}
