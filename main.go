package main

import (
	"errors"
	"io/ioutil"
	"mime/multipart"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/tealeg/xlsx"
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
		for _, row := range sheet.Rows[1:20] {
			var excelRow ExcelRow
			if err := row.ReadStruct(&excelRow); err != nil {
				RespErr(c, err)
				return
			}
			if _, ok := itemsHash[excelRow.ItemCode]; !ok {
				itemsHash[excelRow.ItemCode] = make([]*ExcelRow, 0)
			}
			itemsHash[excelRow.ItemCode] = append(itemsHash[excelRow.ItemCode], &excelRow)
		}
		items, err := youzan.QueryItems()
		if err != nil {
			RespErr(c, err)
			return
		}
		Resp(c, items)
	})
	r.Run()
}

type ExcelRow struct {
	ItemCode string  `json:"item_code" xlsx:"0"`
	ShopCode string  `json:"shop_code" xlsx:"1"`
	ShopName string  `json:"shop_name" xlsx:"2"`
	Quantity float64 `json:"quantity" xlsx:"3"`
	Price    float64 `json:"price" xlsx:"4"`
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
