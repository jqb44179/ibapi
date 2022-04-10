package main

import (
	. "github.com/hadrianl/ibapi"
	"math/rand"
	"time"
)

func main(){
	// internal api log is zap log, you could use GetLogger to get the logger
	// besides, you could use SetAPILogger to set you own log option
	// or you can just use the other logger
	log := GetLogger().Sugar()
	defer log.Sync()
	// implement your own IbWrapper to handle the msg delivered via tws or gateway
	// Wrapper{} below is a default implement which just log the msg
	ic := NewIbClient(&Wrapper{})

	// tcp connect with tws or gateway
	// fail if tws or gateway had not yet set the trust IP
	if err := ic.Connect("127.0.0.1", 7497, 100);err != nil {
		log.Panic("Connect failed:", err)
	}

	// handshake with tws or gateway, send handshake protocol to tell tws or gateway the version of client
	// and receive the server version and connection time from tws or gateway.
	// fail if someone else had already connected to tws or gateway with same clientID
	if err := ic.HandShake();err != nil {
		log.Panic("HandShake failed:", err)
	}
	rand.Seed(time.Now().UnixNano())
	// 合约
	contract := Contract{
		// ContractID:   ic.GetReqID() + rand.Int63n(300000),
		Symbol:       "SPX",
		SecurityType: "OPT",
		Expiry:       "20220413",
		Strike:       4500.0,
		Right:        "C",
		Multiplier:   "100",
		Exchange:     "SMART",
		Currency:     "USD",
		TradingClass: "SPX",
	}
	orderId := ic.GetReqID() + rand.Int63n(100000)
	// ic.ReqContractDetails(orderId, &contract)
	// marketOrder := NewMarketOrder("BUY", 2)
	marketOrder := NewLimitOrder("BUY", 29, 2)
	// order := Order{TotalQuantity: 2, OrderType: "MKT",Action: "BUY"}
	ic.PlaceOrder(orderId, &contract, marketOrder)
	ic.ReqMktData()
	ic.ReqScannerParameters()
	// make some request, msg would be delivered via wrapper.
	// req will not send to TWS or Gateway until ic.Run()
	// you could just call ic.Run() before these
	// ic.ReqCurrentTime()
	// ic.ReqAutoOpenOrders(true)
	// ic.ReqAccountUpdates(true, "")
	// ic.ReqExecutions(ic.GetReqID(), ExecutionFilter{})

	// start to send req and receive msg from tws or gateway after this
	ic.Run()
	ic.LoopUntilDone()
	<-time.After(time.Second * 60)
	ic.Disconnect()
}
