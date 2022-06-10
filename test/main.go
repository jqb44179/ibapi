package main

import (
	"fmt"
	. "github.com/jqb44179/ibapi"
	"time"
)

var Ic *IbClient

type CWrapper struct {
	Wrapper
	undoneContracts map[int64]undoneContract
	optionChain     map[int64]OptionChain
	undyingPrice    float64
}
type OptionChain struct {
	repId                int64
	underlyingContractId int64
	tradingClass         string
	strikes              float64
	expirations          string
	multiplier           string
	optPrice             float64
	right                string
}
type undoneContract struct {
	Contract
	OrderState
	Order
}

func (w CWrapper) TickPrice(reqID int64, tickType int64, price float64, attrib TickAttrib) {
	//fmt.Println("TickPrice:", reqID, tickType, price, attrib)
	typeName := "买价"
	if tickType == 2 {
		typeName = "卖价"
	}
	fprint(fmt.Sprintf("接收到单据价格信息,请求的id是:%d,类型是:%v,价格是:%v", reqID, typeName, price))
}

func (w *CWrapper) Error(reqID int64, errCode int64, errString string) {
	if reqID == -1 {
		return
	}
	fmt.Println("Error:", reqID, errCode, errString)
}

func (w *CWrapper) CompletedOrder(contract *Contract, order *Order, orderState *OrderState) {
	//fmt.Println("CompletedOrder:", contract, order, orderState)
	fmt.Println("CompletedOrder:", contract.Expiry, order.PermID, orderState.Status)
}

/**
* 收到进行中的合约信息
* 执行如下动作:
  1）记录两个订单的情况
*/
func (w CWrapper) OpenOrder(orderID int64, contract *Contract, order *Order, orderState *OrderState) {
	// 收到所有的信息
	fmt.Println("接收到订单状态更变信息,开始检测", contract, order, orderState)
	if contract.Symbol == "SPX" && orderState.Status == "PreSubmitted" && order.OrderType == "STP" && w.undoneContracts[order.PermID].PermID != order.PermID {
		fmt.Println("接收到SPX已提交的订单,加入到操作列表")
		w.undoneContracts[order.PermID] = undoneContract{
			*contract,
			*orderState,
			*order,
		}
		Ic.ReqMktData(order.PermID, contract, "", false, false, nil)
	}
	fmt.Println("接收到订单状态更变信息,结束检测", len(w.undoneContracts))
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
	if contract.Symbol == "SPX" && w.undoneContracts[execution.PermID].PermID == execution.PermID {
		// 从未完成中剔除
		delete(w.undoneContracts, execution.PermID)
		fmt.Println("现存在需要处理的订单数为:", len(w.undoneContracts))
		if len(w.undoneContracts) < 1 {
			fmt.Println("没有需要处理的订单,跳过")
			return
		}
		Ic.ReqGlobalCancel()
		// 开始循环处理
		for _, c := range w.undoneContracts {
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
			fmt.Println("现在对目标进行平仓:", newContract.Symbol+newContract.Expiry, w.undoneContracts[c.PermID].Action, c.Strike, w.undoneContracts[c.PermID].TotalQuantity)
			marketOrder := NewMarketOrder(w.undoneContracts[c.PermID].Action, w.undoneContracts[c.PermID].TotalQuantity)
			Ic.PlaceOrder(Ic.GetReqID(), &newContract, marketOrder)
			fmt.Println("MKT下单成功")
		}
	}
	fmt.Println("接收到订单完成信息,结束检测")
}

func (w CWrapper) OrderStatus(orderID int64, status string, filled float64, remaining float64, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64) {
	if w.undoneContracts[permID].PermID == permID && status == "Cancelled" {
		delete(w.undoneContracts, permID)
		fmt.Println("收到订单取消通知,先存在订单数目为:", len(w.undoneContracts))
	}
}

func (w CWrapper) ContractDetails(reqID int64, conDetails *ContractDetails) {
	// 根据获取的合约详情,获取期权链信息
	if conDetails.Contract.SecurityType == "IND" {
		Ic.ReqSecDefOptParams(reqID, conDetails.Contract.Symbol, "", conDetails.Contract.SecurityType, conDetails.Contract.ContractID)
	}
	// 获取指数价格以及希腊等信息
	if conDetails.Contract.SecurityType == "OPT" {
		Ic.ReqMktData(reqID, &conDetails.Contract, "", false, false, nil)
	}
	fprint(fmt.Sprintf("接收到合约详情信息,合约内容是:%v", conDetails))
}
func (w CWrapper) SecurityDefinitionOptionParameter(reqID int64, exchange string, underlyingContractID int64, tradingClass string, multiplier string, expirations []string, strikes []float64) {

	if exchange == "CBOE" && tradingClass == "SPXW" {
		//if len(w.optionChain) < 0 {
		if w.optionChain == nil {
			w.optionChain = make(map[int64]OptionChain, 20)
		}
		for _, strike := range strikes {
			if strike > 4000.00 && strike < 4200 {
				// 不能超过上线
				if len(w.optionChain) > 49 {
					break
				}
				newReqId := Ic.GetReqID()
				w.optionChain[newReqId] = OptionChain{
					reqID,
					underlyingContractID,
					tradingClass,
					strike,
					expirations[0],
					multiplier,
					0,
					"C",
				}
				index := Contract{
					Symbol:       "SPX",
					Exchange:     exchange,
					SecurityType: "OPT",
					Currency:     "USD",
					TradingClass: tradingClass,
					Expiry:       expirations[0],
					Strike:       strike,
					Right:        "C",
					Multiplier:   multiplier,
				}
				Ic.ReqContractDetails(newReqId, &index)
			}
		}
	}

}

func (w CWrapper) TickOptionComputation(reqID int64, tickType int64, tickAttrib int64, impliedVol float64, delta float64, optPrice float64, pvDiviedn float64, gamma float64, vega float64, theta float64, undPrice float64) {
	//// bid
	//if tickType == 10 {
	//fprint(fmt.Sprintf("接收到买价信息,价格是:%v,标的物价格是:%v", optPrice, undPrice))
	//}
	//// ask
	//if tickType == 11 {
	// 保存指数价格
	//if tickType == 13 {
	//	w.undyingPrice = undPrice
	//}
}

func (w CWrapper) TickSize(reqID int64, tickType int64, size int64) {
	//// bid
	//if tickType == 0 {
	//	fprint(fmt.Sprintf("接收到买价数量,数量是:%v", size))
	//}
	//// ask
	//if tickType == 3 {
	//	fprint(fmt.Sprintf("接收到卖价数量,数量是:%v", size))
	//}
	//if tickType != 0 || tickType != 3 {
	//fprint(fmt.Sprintf("接收到未知的数量,数量是:%v", size))
	//}
}

func main() {
	// internal api log is zap log, you could use GetLogger to get the logger
	// besides, you could use SetAPILogger to set you own log option
	// or you can just use the other logger
	log := GetLogger().Sugar()
	defer log.Sync()
	// implement your own IbWrapper to handle the msg delivered via tws or gateway
	// Wrapper{} below is a default implement which just log the msg
	wrapper := &CWrapper{}
	wrapper.undoneContracts = make(map[int64]undoneContract, 10)
	Ic = NewIbClient(wrapper)

	// tcp connect with tws or gateway
	// fail if tws or gateway had not yet set the trust IP
	if err := Ic.Connect("8.218.27.42", 7497, 100); err != nil {
		//if err := Ic.Connect("127.0.0.1", 7496, 0); err != nil {
		log.Panic("Connect failed:", err)
	}

	// handshake with tws or gateway, send handshake protocol to tell tws or gateway the version of client
	// and receive the server version and connection time from tws or gateway.
	// fail if someone else had already connected to tws or gateway with same clientID
	if err := Ic.HandShake(); err != nil {
		log.Panic("HandShake failed:", err)
	}
	//index := Contract{
	//	Symbol:       "SPX",
	//	Exchange:     "CBOE",
	//	SecurityType: "IND",
	//}
	//Ic.ReqMarketDataType(1)
	//// 获取期权连
	//Ic.ReqContractDetails(Ic.GetReqID(), &index)
	//Ic.ReqMktData(Ic.GetReqID(), &index, "", false, false, nil)
	// start to send req and receive msg from tws or gateway after this
	fprint("开始处理消息")
	//Ic.ReqOpenOrders()
	Ic.ReqAllOpenOrders()
	Ic.Run()
	Ic.LoopUntilDone()

}

func fprint(msg string) {
	fmt.Println(time.Now().Format("2006-01-02 15:04:05"), msg)
}
