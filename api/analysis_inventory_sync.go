package api

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	limitedRequest "github.com/xuyuntech/inventory_sync_go/limited_request"
	"github.com/xuyuntech/inventory_sync_go/youzan"
	"golang.org/x/sync/errgroup"
)

func (a *Api) AnalysisInventorySync(c *gin.Context) {
	if InventorySyncTaskCache == nil {
		RespErr(c, nil, "还没有上传库存文件")
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-strem")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	itemsHash := InventorySyncTaskCache.ItemsHash
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
	logrus.Debugf("items: %d", len(items))

	tasks := make([]*limitedRequest.Task, 0)
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
				logrus.Errorf("有赞里没有门店[%s]", itemExcel.ShopName)
				continue
			}
			itemID := fmt.Sprintf("%d", item.ItemID)
			tasks = append(tasks, &limitedRequest.Task{
				ID:     fmt.Sprintf("%s-%s", itemID, offlineID),
				URL:    "https://open.youzan.com/api/oauthentry/youzan.multistore.goods.sku/3.0.0/get",
				Method: "GET",
				Temp: map[string]interface{}{
					"excelRow": itemExcel,
				},
				Params: map[string]string{
					"num_iid":      itemID,
					"offline_id":   offlineID,
					"access_token": youzan.AccessToken,
				},
			})
		}
	}

	totalTaskNum := len(tasks)

	logrus.Infof("===== Start to fetch goods from youzan. ====")

	lreq := limitedRequest.New(&limitedRequest.Options{
		RequestThresholdPerSecond: 3,
	})

	pingStr := fmt.Sprintf("%x\r\nping", len("ping"))
	io.WriteString(c.Writer, pingStr)
	c.Writer.Flush()

	var (
		g              errgroup.Group
		finishedTaskCh = make(chan *limitedRequest.Task)
		outputCh       = make(chan *goodUpdated)
	)
	done := make(chan struct{})
	notify := c.Writer.CloseNotify()

	g.Go(func() error {
		for {
			logrus.Infof("%s, total: %d", lreq.Status(), totalTaskNum)
			if lreq.FinishedCount() == totalTaskNum {
				close(done)
				return nil
			}
			time.Sleep(time.Second)
		}
	})
	g.Go(func() error {
		for {
			task := <-finishedTaskCh
			logrus.Debugf("task %s finished", task.ID)
			parts := strings.Split(task.ID, "-")
			offlineID := parts[1]
			gd := parseGoodsDetail(task.Body)
			if gd == nil {
				logrus.Errorf("parseGoodsDetail get nil")
				continue
			}
			excelRow, ok := task.Temp["excelRow"].(*ExcelRow)
			if !ok {
				logrus.Errorf("convert task.Temp[excelRow] to *ExcelRow failed")
				continue
			}
			need, err := needYouzanGoodToBeUpdated(excelRow, gd)
			if err != nil {
				logrus.Errorf("needYouzanGoodToBeUpdated failed: %v", err)
			}
			if !need {
				logrus.Debugf("no need update")
				continue
			}
			offlineIDD, _ := strconv.ParseInt(offlineID, 10, 64)
			numIIDD, _ := strconv.ParseInt(gd.NumIID, 10, 64)

			skus := make([]*goodUpdatedSku, 0)
			for _, sku := range gd.Skus {
				properties, err := sku.FormatProperties()
				if err != nil {
					logrus.Errorf("解析 properties_name_json 出错: %v", err)
					continue
				}
				if len(properties) != 1 {
					logrus.Errorf("商品 [%d][%s] 的规格不唯一", gd.NumIID, gd.Title)
					continue
				}
				property := properties[0]

				skus = append(skus, &goodUpdatedSku{
					ID:         sku.SkuID,
					K:          property.K,
					V:          property.V,
					Price:      sku.Price,
					Quantity:   sku.Quantity,
					ToPrice:    excelRow.Price,
					ToQuantity: excelRow.Quantity,
				})
			}
			gu := &goodUpdated{
				ItemID:      numIIDD,
				ItemTitle:   gd.Title,
				ItemNo:      gd.OuterID,
				OfflineID:   offlineIDD,
				OfflineName: excelRow.ShopName,
				Skus:        skus,
			}
			logrus.Debugf("append to chan output: %+v", gu)
			outputCh <- gu
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
			case <-time.After(time.Hour):
				io.WriteString(c.Writer, fmt.Sprintf("%x\r\n%s", len("timeout"), "timeout"))
				c.Writer.Flush()
				lreq.Stop()
				return nil
			case <-done:
				io.WriteString(c.Writer, fmt.Sprintf("%x\r\n%s", len("eof"), "eof"))
				c.Writer.Flush()
				lreq.Stop()
				return nil
			case <-notify:
				lreq.Stop()
				return nil
			case out := <-outputCh:
				b, _ := json.Marshal(out)
				s := string(b)
				logrus.Debugf("repsonse write: %s", s)
				io.WriteString(c.Writer, fmt.Sprintf("%x\r\n%s", len(s), s))
				c.Writer.Flush()
			case <-time.After(time.Second * 30):
				io.WriteString(c.Writer, pingStr)
				c.Writer.Flush()
			case task := <-results:
				finishedTaskCh <- task
			}
		}
	})

	if err := g.Wait(); err != nil {
		RespErr(c, err)
	}

}
