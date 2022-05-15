package main

import (
	"fmt"
	. "github.com/hadrianl/ibapi"
)

var Ic *IbClient
var actionStatus bool

var undoneContracts map[int64]undoneContract

type CWrapper struct {
	Wrapper
}
type undoneContract struct {
	Contract
	OrderState
	Order
}

func (w CWrapper) TickPrice(reqID int64, tickType int64, price float64, attrib TickAttrib) {
	fmt.Println("TickPrice:", reqID, tickType, price, attrib)
}

func (w *CWrapper) Error(reqID int64, errCode int64, errString string) {
	if reqID == -1 {
		return
	}
	fmt.Println("Error:", reqID, errCode, errString)
	//log.With(zap.Int64("reqID", reqID)).Info("<Error>",
	//	zap.Int64("errCode", errCode),
	//	zap.String("errString", errString),
	//)
}

func (w *CWrapper) CompletedOrder(contract *Contract, order *Order, orderState *OrderState) {
	fmt.Println("CompletedOrder:", contract, order, orderState)
}

/**
* 收到进行中的合约信息
* 执行如下动作:
  1）记录两个订单的情况
*/
func (w CWrapper) OpenOrder(orderID int64, contract *Contract, order *Order, orderState *OrderState) {
	fmt.Println(contract, order, orderState)
	// 收到所有的信息
	fmt.Println("接收到订单状态信息,开始检测", contract, order, orderState)
	if contract.Symbol == "SPX" && orderState.Status == "Submitted" {
		fmt.Println("接收到SPX已提交的订单,加入到undoneContracts")
		undoneContracts[contract.ContractID] = undoneContract{
			*contract,
			*orderState,
			*order,
		}
	}
	fmt.Println("接收到订单状态信息,结束检测", len(undoneContracts))
}

/**
 * 收到订单完成回调
 * 执行以下动作:
3、监听到完成订单的回调信息
    1）获取完成订单的信息
    2）将另一个订单取消
    3）将另一个订单反向MKT下单
*/
func (w CWrapper) ExecDetails(reqID int64, contract *Contract, execution *Execution) {
	fmt.Println("接收到订单完成信息,开始检测", reqID, contract, execution)
	if contract.Symbol == "SPX" && undoneContracts[contract.ContractID].ContractID == contract.ContractID {
		//_ := undoneContracts[contract.ContractID]
		// 从未完成中剔除
		delete(undoneContracts, contract.ContractID)
		fmt.Println("接收到SPX已完成的订单,并且在记录的未完成的订单中,现在开始取消订单")
		Ic.ReqGlobalCancel()
		// 开始循环处理
		for _, c := range undoneContracts {
			newContract := Contract{
				Symbol:       c.Symbol,
				SecurityType: c.SecurityType,
				Expiry:       c.Expiry,
				Strike:       c.Strike,
				Right:        c.Right,
				Multiplier:   c.Multiplier,
				Exchange:     c.Exchange,
				Currency:     c.Currency,
				TradingClass: c.TradingClass,
			}
			actionType := ""
			if undoneContracts[c.ContractID].OrderType == "SELL" {
				actionType = "BUY"
			} else {
				actionType = "SELL"
			}
			fmt.Println("现在对目标进行平仓:", newContract.Symbol+newContract.Expiry, actionType, undoneContracts[c.ContractID].TotalQuantity)
			marketOrder := NewMarketOrder(actionType, undoneContracts[c.ContractID].TotalQuantity)
			Ic.PlaceOrder(Ic.GetReqID(), &newContract, marketOrder)
			fmt.Println("MKT下单成功")
		}
	}
	fmt.Println("接收到订单完成信息,结束检测")
}

// func (w CWrapper) OrderStatus(orderID int64, status string, filled float64, remaining float64, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64) {
// 	log.With(zap.Int64("orderID", orderID)).Info("<OrderStatus>",
// 		zap.String("status", status),
// 		zap.Float64("filled", filled),
// 		zap.Float64("remaining", remaining),
// 		zap.Float64("avgFillPrice", avgFillPrice),
// 	)
// }

func main() {
	actionStatus = false
	undoneContracts = make(map[int64]undoneContract, 10)
	// internal api log is zap log, you could use GetLogger to get the logger
	// besides, you could use SetAPILogger to set you own log option
	// or you can just use the other logger
	log := GetLogger().Sugar()
	defer log.Sync()
	// implement your own IbWrapper to handle the msg delivered via tws or gateway
	// Wrapper{} below is a default implement which just log the msg
	wrapper := &CWrapper{}
	Ic = NewIbClient(wrapper)

	// tcp connect with tws or gateway
	// fail if tws or gateway had not yet set the trust IP
	if err := Ic.Connect("127.0.0.1", 7497, 0); err != nil {
		log.Panic("Connect failed:", err)
	}

	// handshake with tws or gateway, send handshake protocol to tell tws or gateway the version of client
	// and receive the server version and connection time from tws or gateway.
	// fail if someone else had already connected to tws or gateway with same clientID
	if err := Ic.HandShake(); err != nil {
		log.Panic("HandShake failed:", err)
	}
	// 合约
	//contract := Contract{
	//	// ContractID:   Ic.GetReqID() + rand.Int63n(300000),
	//	Symbol:       "SPX",
	//	SecurityType: "OPT",
	//	Expiry:       "20220519",
	//	Strike:       200.0,
	//	Right:        "C",
	//	Multiplier:   "100",
	//	Exchange:     "CBOE",
	//	Currency:     "USD",
	//	TradingClass: "SPX",
	//}

	// orderId := Ic.GetReqID() + rand.Int63n(100000)
	// Ic.ReqContractDetails(orderId, &contract)
	// marketOrder := NewMarketOrder("BUY", 2)
	//marketOrder := NewLimitOrder("BUY", 29, 2)
	// order := Order{TotalQuantity: 2, OrderType: "MKT",Action: "BUY"}
	//Ic.PlaceOrder(orderId, &contract, marketOrder)
	//Ic.ReqMktData(orderId, &contract, "", false, false, nil)
	//Ic.ReqScannerParameters()
	// make some request, msg would be delivered via wrapper.
	// req will not send to TWS or Gateway until Ic.Run()
	// you could just call Ic.Run() before these
	//Ic.ReqCurrentTime()
	// Ic.ReqAutoOpenOrders(true)
	// Ic.ReqAccountUpdates(true, "")
	// Ic.ReqExecutions(Ic.GetReqID(), ExecutionFilter{})
	// Ic.ReqAllOpenOrders()
	//tags := "EquityWithLoanValue"
	//Ic.ReqAccountSummary(Ic.GetReqID(), "All", tags)
	Ic.ReqOpenOrders()
	// Ic.ReqCompletedOrders(false)
	// Ic.ReqAutoOpenOrders(false)
	// start to send req and receive msg from tws or gateway after this
	fmt.Println("开始处理消息")
	Ic.Run()
	Ic.LoopUntilDone()

}
