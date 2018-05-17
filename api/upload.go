package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/tealeg/xlsx"
	limitedRequest "github.com/xuyuntech/inventory_sync_go/limited_request"
	"github.com/xuyuntech/inventory_sync_go/youzan"
	"golang.org/x/sync/errgroup"
)

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
	// 获取所有门店
	offlines, err := youzan.QueryOfflines()
	if err != nil {
		RespErr(c, err, "获取 offlines 失败")
		return
	}
	// 获取所有有赞商品
	items, err := youzan.QueryItems()
	if err != nil {
		RespErr(c, err)
		return
	}

	tasks := make([]*limitedRequest.Task, 0)
	logrus.Debugf("items: %d", len(items))
	for _, item := range items {
		if item.ItemNO == "" {
			logrus.Debugf("items (%s) has no item_no", item.Title)
			continue
		}
		// 到 excel 里找 item.ItemNo 对应的所有条目
		itemsExcel, ok := itemsHash[item.ItemNO]
		if !ok {
			logrus.Debugf("itemsExcel %s (%s) no found", item.ItemNO, item.Title)
			continue
		}
		for _, itemExcel := range itemsExcel {
			offlineID, err := offlines.GetIDByName(itemExcel.ShopName)
			if err != nil {
				logrus.Errorf("excel 里没有门店 %s", itemExcel.ShopName)
				continue
			}
			itemID := fmt.Sprintf("%d", item.ItemID)
			offlineIDStr := fmt.Sprintf("%d", offlineID)
			tasks = append(tasks, &limitedRequest.Task{
				ID:     fmt.Sprintf("%s-%s", itemID, offlineIDStr),
				URL:    "https://open.youzan.com/api/oauthentry/youzan.multistore.goods.sku/3.0.0/get",
				Method: "GET",
				Params: map[string]string{
					"num_iid":      itemID,
					"offline_id":   offlineIDStr,
					"access_token": youzan.AccessToken,
				},
			})
		}
	}

	logrus.Infof("===== Start to fetch goods from youzan. ====")

	lreq := limitedRequest.New(&limitedRequest.Options{
		RequestThresholdPerSecond: 5,
	})

	c.Writer.Header().Set("Content-Type", "text/event-strem")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	var (
		g             errgroup.Group
		goodsDetailCh = make(chan *youzan.GoodsDetail)
	)

	notify := c.Writer.CloseNotify()

	g.Go(func() error {
		for {
			gd := <-goodsDetailCh

		}
	})

	g.Go(func() error {
		logrus.Debugf("tasks 总数 %d", len(tasks))
		lreq.Add(tasks)
		return nil
	})
	g.Go(func() error {
		err := lreq.Start()
		logrus.Debugf("LimitedRequest ended with err: %v", err)
		return err
	})

	g.Go(func() error {
		results := lreq.Results()
		for {
			select {
			case <-notify:
				lreq.Stop()
				return nil
			case result := <-results:
				if gd := parseGoodsDetail(result); gd != nil {
					goodsDetailCh <- gd
				}
				// logrus.Debugf("result-> %s, %s\r", string(result), lreq.Status())
			}
		}
	})

	if err := g.Wait(); err != nil {
		RespErr(c, err)
	}
}

func parseGoodsDetail(b []byte) *youzan.GoodsDetail {
	gd := &youzan.GoodsDetail{}
	if err := json.Unmarshal(b, gd); err != nil {
		logrus.Errorf("转换 GoodsDetail 出错, b: (%s)", string(b))
		return nil
	}
	return gd
}

func needYouzanGoodToBeUpdated(excelRow *ExcelRow, gd *youzan.GoodsDetail) (bool, error) {
	if len(gd.Skus) <= 0 {
		return false, fmt.Errorf("有赞商品 [%s] 没有设置 sku", gd.Title)
	}
	sku := gd.Skus[0]
	if sku.OuterID != gd.OuterID {
		return false, fmt.Errorf("有赞商品 [%s] 第一个 sku 的商品编码设置不正确")
	}

	if sku.Price != excelRow.Price || sku.Quantity != excelRow.Quantity {
		return true, nil
	}
}
