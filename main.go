package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Sirupsen/logrus"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/tealeg/xlsx"
	limitedRequest "github.com/xuyuntech/inventory_sync_go/limited_request"
	"github.com/xuyuntech/inventory_sync_go/youzan"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "X-Requested-With", "X-Access-Token"},
		AllowCredentials: false,
		AllowAllOrigins:  true,
		MaxAge:           12 * time.Hour,
	}))
	r.POST("/upload", func(c *gin.Context) {
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
				itemID := fmt.Sprintf("%d", item.ItemID)
				tasks = append(tasks, &limitedRequest.Task{
					ID:     fmt.Sprintf("%s-%s", itemID, itemExcel.OfflineID),
					URL:    "https://open.youzan.com/api/oauthentry/youzan.multistore.goods.sku/3.0.0/get",
					Method: "GET",
					Params: map[string]string{
						"num_iid":      itemID,
						"offline_id":   itemExcel.OfflineID,
						"access_token": youzan.AccessToken,
					},
				})
			}
		}

		logrus.Infof("===== Start to fetch goods from youzan. ====")

		lreq := limitedRequest.New(&limitedRequest.Options{
			RequestThresholdPerSecond: 5,
		})

		var g errgroup.Group

		notify := c.Writer.CloseNotify()

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
					logrus.Debugf("result-> %s, %s\r", string(result), lreq.Status())
				}
			}
		})

		if err := g.Wait(); err != nil {
			RespErr(c, err)
		}
	})
	r.Run()
}

type ExcelRow struct {
	ItemNO    string  `json:"item_no" xlsx:"0"`
	OfflineID string  `json:"offline_id" xlsx:"1"`
	ShopName  string  `json:"shop_name" xlsx:"2"`
	Quantity  float64 `json:"quantity" xlsx:"3"`
	Price     float64 `json:"price" xlsx:"4"`
}

func RespErr(c *gin.Context, err error, msg ...string) {
	results := map[string]interface{}{
		"status": 1,
		"err":    err,
	}
	if len(msg) >= 1 {
		results["msg"] = msg[0]
	}
	c.JSON(200, results)
}

func Resp(c *gin.Context, results interface{}) {
	c.JSON(200, map[string]interface{}{
		"status": 0,
		"data":   results,
	})
}
