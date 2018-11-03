package tokenswap

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/dynamicgo/config"
	"github.com/dynamicgo/slf4go"
	"github.com/go-xorm/xorm"
	"github.com/inwecrypto/ethdb"
	"github.com/inwecrypto/ethgo"
	"github.com/inwecrypto/ethgo/erc20"
	ethkeystore "github.com/inwecrypto/ethgo/keystore"
	ethmath "github.com/inwecrypto/ethgo/math"
	ethrpc "github.com/inwecrypto/ethgo/rpc"
	ethtx "github.com/inwecrypto/ethgo/tx"
	"github.com/inwecrypto/gomq"
	"github.com/inwecrypto/neodb"
	neokeystore "github.com/inwecrypto/neogo/keystore"
	"github.com/inwecrypto/neogo/nep5"
	neorpc "github.com/inwecrypto/neogo/rpc"
	neotx "github.com/inwecrypto/neogo/tx"
)

const ETH_TNC_DECIAMLS = 8

var ethTxChan chan string

func init() {
	ethTxChan = make(chan string, 10000)
}

// Monitor neo/eth tx event monitor
type Monitor struct {
	slf4go.Logger
	neomq               gomq.Consumer
	ethmq               gomq.Consumer
	tokenswapdb         *xorm.Engine
	ethdb               *xorm.Engine
	neodb               *xorm.Engine
	tncOfNEO            string
	tncOfETH            string
	ETHKeyAddress       string
	NEOKeyAddress       string
	ethClient           *ethrpc.Client
	neoClient           *neorpc.Client
	config              *config.Config
	neo2ethtax          float64 // 转账费率
	eth2neotax          float64
	ethConfirmCount     int64
	ethGetBlockInterval int64

	neoConfirmCount     int64
	neoGetBlockInterval int64
}

// NewMonitor create new monitor
func NewMonitor(conf *config.Config, neomq, ethmq gomq.Consumer) (*Monitor, error) {

	tokenswapdb, err := createEngine(conf, "tokenswapdb")

	if err != nil {
		return nil, fmt.Errorf("create tokenswap db engine error %s", err)
	}

	ethdb, err := createEngine(conf, "ethdb")

	if err != nil {
		return nil, fmt.Errorf("create eth db engine error %s", err)
	}

	neodb, err := createEngine(conf, "neodb")

	if err != nil {
		return nil, fmt.Errorf("create neo db engine error %s", err)
	}

	ethKey, err := readETHKeyStore(conf, "eth.keystore", conf.GetString("eth.keystorepassword", ""))

	if err != nil {
		return nil, fmt.Errorf("create neo db engine error %s", err)
	}

	neoKey, err := readNEOKeyStore(conf, "neo.keystore", conf.GetString("neo.keystorepassword", ""))

	if err != nil {
		return nil, fmt.Errorf("create neo db engine error %s", err)
	}

	neo2ethtax, err := strconv.ParseFloat(conf.GetString("tokenswap.neo2ethtax", "0.001"), 64)
	if err != nil {
		return nil, fmt.Errorf("ParseFloat neo2ethtax error %s", err)
	}

	eth2neotax, err := strconv.ParseFloat(conf.GetString("tokenswap.eth2neotax", "0.001"), 64)
	if err != nil {
		return nil, fmt.Errorf("ParseFloat eth2neotax error %s", err)
	}

	return &Monitor{
		Logger:              slf4go.Get("tokenswap-server"),
		neomq:               neomq,
		ethmq:               ethmq,
		tokenswapdb:         tokenswapdb,
		ethdb:               ethdb,
		neodb:               neodb,
		tncOfETH:            conf.GetString("eth.tnc", ""),
		tncOfNEO:            conf.GetString("neo.tnc", ""),
		ETHKeyAddress:       strings.ToLower(ethKey.Address),
		NEOKeyAddress:       neoKey.Address,
		ethClient:           ethrpc.NewClient(conf.GetString("eth.node", "")),
		neoClient:           neorpc.NewClient(conf.GetString("neo.node", "")),
		neo2ethtax:          neo2ethtax,
		eth2neotax:          eth2neotax,
		config:              conf,
		ethConfirmCount:     conf.GetInt64("tokenswap.ethConfirmCount", 12),
		ethGetBlockInterval: conf.GetInt64("tokenswap.ethGetBlockInterval", 20),
		neoConfirmCount:     conf.GetInt64("tokenswap.neoConfirmCount", 12),
		neoGetBlockInterval: conf.GetInt64("tokenswap.neoGetBlockInterval", 10),
	}, nil
}

func readETHKeyStore(conf *config.Config, name string, password string) (*ethkeystore.Key, error) {
	data, err := json.Marshal(conf.Get(name))

	if err != nil {
		return nil, err
	}

	return ethkeystore.ReadKeyStore(data, password)
}

func readNEOKeyStore(conf *config.Config, name string, password string) (*neokeystore.Key, error) {
	data, err := json.Marshal(conf.Get(name))

	if err != nil {
		return nil, err
	}

	return neokeystore.ReadKeyStore(data, password)
}

func createEngine(conf *config.Config, name string) (*xorm.Engine, error) {
	driver := conf.GetString(fmt.Sprintf("%s.driver", name), "postgres")
	datasource := conf.GetString(fmt.Sprintf("%s.datasource", name), "")

	return xorm.NewEngine(driver, datasource)
}

// NEOAddress .
func (monitor *Monitor) NEOAddress() string {
	return monitor.NEOKeyAddress
}

func (monitor *Monitor) ethMonitor() {
	for {
		select {
		case message, ok := <-monitor.ethmq.Messages():
			if ok {
				if monitor.handleETHMessage(string(message.Key())) {
					monitor.ethmq.Commit(message)
				}
			}
		case err := <-monitor.ethmq.Errors():
			monitor.ErrorF("ethmq err, %s", err)
		}
	}
}

func (monitor *Monitor) neoMonitor() {
	for {
		select {
		case message, ok := <-monitor.neomq.Messages():
			if ok {
				if monitor.handleNEOMessage(string(message.Key())) {
					monitor.neomq.Commit(message)
				}
			}
		case err := <-monitor.neomq.Errors():
			monitor.ErrorF("neomq err, %s", err)
		}
	}
}

// Run .
func (monitor *Monitor) Run() {
	go monitor.ethMonitor()
	go monitor.neoMonitor()
	go monitor.NeoSendMoniter()
	go monitor.EthSendMoniter()
}

func (monitor *Monitor) handleNEOMessage(txid string) bool {
	//	monitor.DebugF("tokenswap handle neo tx %s", txid)

	neoTxs := make([]neodb.Tx, 0)

	err := monitor.neodb.Where(` "t_x" = ?`, txid).Find(&neoTxs)

	if err != nil {
		monitor.ErrorF("1 handle neo tx %s error, %s", txid, err)
		return false
	}

	for _, neoTx := range neoTxs {
		if neoTx.From == monitor.NEOKeyAddress && neoTx.Asset == monitor.tncOfNEO {
			monitor.DebugF("1 checked neo tx (%s) from:%s  to:%s  value:%s  asset:%s", neoTx.TX, neoTx.From, neoTx.To, neoTx.Value, monitor.tncOfNEO)

			monitor.CheckNeoBlockNumber(neoTx.Block)

			order, err := monitor.getOrderByToAddress(neoTx.To, neoTx.Value, neoTx.CreateTime, ` "in_tx" != '' and  "out_tx" = '' `)

			if err != nil {
				monitor.ErrorF("2 handle order in neo tx %s error, %s", txid, err)
				return false
			}

			log := &Log{
				TX:         order.TX,
				CreateTime: time.Now(),
				Content:    fmt.Sprintf("release TNC to %s success, tx: %s", neoTx.To, txid),
			}

			order.OutTx = neoTx.TX
			order.CompletedTime = time.Now()

			if err := monitor.insertLogAndUpdate(log, order, "out_tx", "completed_time"); err != nil {
				monitor.ErrorF("3 handle neo tx %s error, %s", txid, err)
				return false
			}

			if err := monitor.updateSendOrderOutTxStatus(neoTx.TX); err != nil {
				monitor.ErrorF("neo updateSendOrderOutTxStatus error, %s", txid, err)
				return false
			}

			return true

		} else if neoTx.To == monitor.NEOKeyAddress && neoTx.Asset == monitor.tncOfNEO {

			monitor.DebugF("2 checked neo tx (%s) from:%s  to:%s  value:%s  asset:%s", neoTx.TX, neoTx.From, neoTx.To, neoTx.Value, monitor.tncOfNEO)

			monitor.CheckNeoBlockNumber(neoTx.Block)

			order, err := monitor.getOrderByFromAddress(neoTx.From, neoTx.Value, neoTx.CreateTime, ` "in_tx" = '' and  "out_tx" = '' `)

			if err != nil {
				monitor.ErrorF("4 handle order in neo tx %s error, %s", txid, err)
				return false
			}

			order.InTx = neoTx.TX

			log := &Log{
				TX:         order.TX,
				CreateTime: time.Now(),
				Content:    fmt.Sprintf("recv TNC from %s success, tx: %s", neoTx.From, txid),
			}

			if err := monitor.insertLogAndUpdate(log, order, "in_tx"); err != nil {
				monitor.ErrorF("5 handle neo tx %s error, %s", txid, err)
				return false
			}

			monitor.DebugF("in_tx:(%s)", neoTx.TX)

			if err := monitor.insertSendOrder(order, 1); err != nil {
				monitor.ErrorF("insertSendOrder returned :%s", err.Error())
				return false
			}

			monitor.DebugF("insertSendOrder  end %d", order.ID)

			//			if err := monitor.sendETH(order); err != nil {
			//				monitor.ErrorF("handle neo tx %s -- send TNC error %s", txid, err)
			//				return false
			//			}

			//			if err := monitor.insertLogAndUpdate(nil, order, "tax_cost", "send_value"); err != nil {
			//				monitor.ErrorF("handle neo tx %s error, %s", txid, err)
			//				return false
			//			}

			return true
		}
	}

	return true
}

func (monitor *Monitor) parseEthValue(value string) string {
	bigValue, _ := ethmath.ParseBig256(value)
	v := bigValue.Int64()

	return fmt.Sprint(v)
}

func (monitor *Monitor) handleETHMessage(txid string) bool {
	//	monitor.DebugF("tokenswap handle eth tx %s", txid)

	ethTx := new(ethdb.TableTx)

	ok, err := monitor.ethdb.Where("t_x = ?", txid).Get(ethTx)

	if err != nil {
		monitor.ErrorF("handle eth tx %s error, %s", txid, err)
		return false
	}

	if !ok {
		monitor.WarnF("1 handle eth tx %s -- not found", txid)
		return true
	}

	if ethTx.From == monitor.ETHKeyAddress && ethTx.Asset == monitor.tncOfETH {

		ethTxChan <- ethTx.TX

		if !monitor.CheckEthBlockNumber(ethTx.TX, ethTx.Blocks) {
			monitor.ErrorF("CheckEthBlockNumber can not find tx :%s", ethTx.TX)
			return true
		}

		// complete order
		value := monitor.parseEthValue(ethTx.Value)

		monitor.DebugF("1 checked eth tx (%s) from:%s  to:%s  value:%s  asset:%s", ethTx.TX, ethTx.From, ethTx.To, value, monitor.tncOfETH)

		order, err := monitor.getOrderByToAddress(ethTx.To, value, ethTx.CreateTime, ` "in_tx" != '' and  "out_tx" = '' `)

		if err != nil {
			monitor.ErrorF("2 handle order in eth tx %s error, %s", txid, err)
			return false
		}

		log := &Log{
			TX:         order.TX,
			CreateTime: time.Now(),
			Content:    fmt.Sprintf("release TNC to %s success, tx: %s", ethTx.To, txid),
		}

		order.OutTx = ethTx.TX
		order.CompletedTime = time.Now()

		if err := monitor.insertLogAndUpdate(log, order, "out_tx", "completed_time"); err != nil {
			monitor.ErrorF("3 handle eth tx %s error, %s", txid, err)
			return false
		}

		if err := monitor.updateSendOrderOutTxStatus(ethTx.TX); err != nil {
			monitor.ErrorF("eth updateSendOrderOutTxStatus error, %s", txid, err)
			return false
		}

		return true

	} else if ethTx.To == monitor.ETHKeyAddress && ethTx.Asset == monitor.tncOfETH {

		if !monitor.CheckEthBlockNumber(ethTx.TX, ethTx.Blocks) {
			monitor.ErrorF("CheckEthBlockNumber can not find tx :%s", ethTx.TX)
			return true
		}

		value := monitor.parseEthValue(ethTx.Value)

		monitor.DebugF("2 checked eth tx (%s)  from:%s  to:%s  value:%s  asset:%s", ethTx.TX, ethTx.From, ethTx.To, value, monitor.tncOfETH)

		order, err := monitor.getOrderByFromAddress(ethTx.From, value, ethTx.CreateTime, ` "in_tx" = '' and  "out_tx" = '' `)

		if err != nil {
			monitor.ErrorF("4 handle order in eth tx  %s error, %s", txid, err)
			return false
		}

		order.InTx = ethTx.TX

		log := &Log{
			TX:         order.TX,
			CreateTime: time.Now(),
			Content:    fmt.Sprintf("recv TNC from %s success, tx: %s", ethTx.From, txid),
		}

		if err := monitor.insertLogAndUpdate(log, order, "in_tx"); err != nil {
			monitor.ErrorF("5 handle eth tx %s error, %s", txid, err)
			return false
		}

		if err := monitor.insertSendOrder(order, 2); err != nil {
			return false
		}

		//		if err := monitor.sendNEO(order); err != nil {
		//			monitor.ErrorF("handle eth tx %s -- send TNC error %s", txid, err)
		//			return false
		//		}

		//		if err := monitor.insertLogAndUpdate(nil, order, "tax_cost", "send_value"); err != nil {
		//			monitor.ErrorF("handle eth tx %s error, %s", txid, err)
		//			return false
		//		}

		return true
	}

	return true
}

func (monitor *Monitor) insertLogAndUpdate(log *Log, order *Order, cls ...string) error {

	session := monitor.tokenswapdb.NewSession()

	defer session.Close()

	var err error
	if log != nil {
		_, err = session.InsertOne(log)

		if err != nil {
			session.Rollback()
			monitor.ErrorF("insert order(%s,%s,%s) log error, %s", order.From, order.To, order.Value, err)
			return err
		}
	}

	_, err = session.Where(` "t_x" = ?`, order.TX).Cols(cls...).Update(order)

	if err != nil {
		session.Rollback()
		monitor.ErrorF("update order(%s,%s,%s) error, %s", order.From, order.To, order.Value, err)
		return err
	}

	return nil
}

func (monitor *Monitor) sendNEO(order *Order) (string, error) {

	amount, b := ethmath.ParseUint64(order.Value)
	if !b {
		monitor.ErrorF("ParseUint64  %s  err  ", order.Value)
		return "", errors.New("ParseUint64 err")
	}

	taxAmount := int64(float64(amount) * monitor.eth2neotax)

	order.TaxCost = fmt.Sprint(taxAmount)

	transferValue := big.NewInt(int64(amount) - taxAmount)

	order.SendValue = fmt.Sprint(transferValue.Int64())

	key, err := readNEOKeyStore(monitor.config, "neo.keystore", monitor.config.GetString("neo.keystorepassword", ""))

	if err != nil {
		monitor.ErrorF("read neo key  error %s", err)
		return "", err
	}

	from := ToInvocationAddress(key.Address)

	to := ToInvocationAddress(order.To)

	scriptHash, err := hex.DecodeString(strings.TrimPrefix(monitor.tncOfNEO, "0x"))
	if err != nil {
		monitor.ErrorF("DecodeString  error %s", err)
		return "", err
	}

	scriptHash = reverseBytes(scriptHash)

	bytesOfFrom, err := hex.DecodeString(from)
	if err != nil {
		monitor.ErrorF("DecodeString  error %s", err)
		return "", err
	}

	bytesOfFrom = reverseBytes(bytesOfFrom)

	bytesOfTo, err := hex.DecodeString(to)
	if err != nil {
		monitor.ErrorF("DecodeString  error %s", err)
		return "", err
	}

	bytesOfTo = reverseBytes(bytesOfTo)

	script, err := nep5.Transfer(scriptHash, bytesOfFrom, bytesOfTo, transferValue)

	if err != nil {
		monitor.ErrorF("Transfer  error:%s,  to:%s  value: %s", err, order.To, order.Value)
		return "", err
	}

	nonce, _ := time.Now().MarshalBinary()

	tx := neotx.NewInvocationTx(script, 0, bytesOfFrom, nonce)

	rawtx, txId, err := tx.Tx().Sign(key.PrivateKey)

	if err != nil {
		monitor.ErrorF("Sign  error:%s,   to:%s  value: %s", err, order.To, order.Value)
		return "", err
	}

	status, err := monitor.neoClient.SendRawTransaction(rawtx)

	if err != nil || !status {
		monitor.ErrorF("SendRawTransaction  error:%s,  to:%s  value: %s", err, order.To, order.Value)
		return "", err
	}

	monitor.InfoF("send NEO-TNC success tx: %s  from: %s to: %s value: %s ", txId, monitor.NEOKeyAddress, order.To, order.Value)

	return "0x" + txId, nil
}

func (monitor *Monitor) sendETH(order *Order) (string, error) {

	amount, b := ethmath.ParseUint64(order.Value)
	if !b {
		monitor.ErrorF("ParseUint64  %s  err  ", order.Value)
		return "", errors.New("ParseUint64 err")
	}

	taxAmount := int64(float64(amount) * monitor.neo2ethtax)

	order.TaxCost = fmt.Sprint(taxAmount)

	transferValue := big.NewInt(int64(amount) - taxAmount)

	order.SendValue = fmt.Sprint(transferValue.Int64())

	codes, err := erc20.Transfer(order.To, hex.EncodeToString(transferValue.Bytes()))
	if err != nil {
		monitor.ErrorF("get erc20.Transfer(%s,%s) code err: %s ", order.To, order.Value, err.Error())
		return "", err
	}

	gasLimits := big.NewInt(65000)
	gasPrice := ethgo.NewValue(big.NewFloat(20), ethgo.Shannon)

	nonce, err := monitor.ethClient.Nonce(monitor.ETHKeyAddress)
	if err != nil {
		monitor.ErrorF("get Nonce   (%s,%s)  err: %s ", order.To, order.Value, err.Error())
		return "", err
	}

	if order.Retry > 0 {
		if order.Retry > 10 {
			order.Retry = 10
		}
		gasPrice = ethgo.NewValue(big.NewFloat(float64(20)*math.Pow(1.1, float64(order.Retry))), ethgo.Shannon)
	}

	ethKey, err := readETHKeyStore(monitor.config, "eth.keystore", monitor.config.GetString("eth.keystorepassword", ""))

	if err != nil {
		monitor.ErrorF("read eth key error %s", err)
		return "", err
	}

	ntx := ethtx.NewTx(nonce, monitor.tncOfETH, nil, gasPrice, gasLimits, codes)
	err = ntx.Sign(ethKey.PrivateKey)
	if err != nil {
		monitor.ErrorF("Sign  (%s,%s)  err: %s ", order.To, order.Value, err.Error())
		return "", err
	}

	rawtx, err := ntx.Encode()
	if err != nil {
		monitor.ErrorF("Encode  (%s,%s)  err: %s ", order.To, order.Value, err.Error())
		return "", err
	}

	tx, err := monitor.ethClient.SendRawTransaction(rawtx)
	if err != nil {
		monitor.ErrorF("SendRawTransaction  (%s,%s)  err: %s ", order.To, order.Value, err.Error())
		return "", err
	}

	monitor.InfoF("send ETH-TNC success tx: %s  from: %s to: %s value: %s ", tx, monitor.ETHKeyAddress, order.To, order.Value)

	return tx, nil
}

// getOrder .
func (monitor *Monitor) getOrderByToAddress(to, value string, createTime time.Time, where string) (*Order, error) {

	order := new(Order)

	if where != "" {
		where = " and " + where
	}

	ok, err := monitor.tokenswapdb.Where(
		`"to" = ? and "send_value" = ? and "create_time" < ?  `+where, to, value, createTime).Get(order)

	if err != nil {
		monitor.ErrorF("query to order(%s,%s,%s) error, %s", to, value, where, err)
		return nil, err
	}

	if !ok {
		monitor.ErrorF("query to order(%s,%s,%s) not found", to, value, where)
		return nil, errors.New("not found")
	}

	return order, nil
}

func (monitor *Monitor) getOrderByFromAddress(from, value string, createTime time.Time, where string) (*Order, error) {

	order := new(Order)

	if where != "" {
		where = " and " + where
	}

	ok, err := monitor.tokenswapdb.Where(
		`"from" = ? and "value" =? and "create_time" < ? `+where, from, value, createTime).Get(order)

	if err != nil {
		monitor.ErrorF("query from order(%s,%s,%s) error, %s", from, value, where, err)
		return nil, err
	}

	if !ok {
		monitor.ErrorF("query from order(%s,%s,%s) not found", from, value, where)
		return nil, errors.New("not found")
	}

	return order, nil
}

func (monitor *Monitor) insertSendOrder(order *Order, ty int32) error {

	monitor.DebugF("insertSendOrder %d", order.ID)

	if order == nil {
		return nil
	}

	sendOrder := new(SendOrder)
	sendOrder.OrderTx = order.TX
	sendOrder.To = order.To
	sendOrder.Value = order.Value
	sendOrder.Status = 0
	sendOrder.ToType = ty
	sendOrder.CreateTime = time.Now()

	affected, err := monitor.tokenswapdb.InsertOne(sendOrder)
	if err != nil {
		monitor.ErrorF("insert new send order err: %s, %d", err.Error(), order.ID)
	}

	monitor.InfoF("insert new send order:%d,affected:%d", order.ID, affected)

	return err
}

func (monitor *Monitor) getSendOrder(ty int32) ([]*SendOrder, error) {
	orders := make([]*SendOrder, 0)

	err := monitor.tokenswapdb.Where(`( status = 0 or status = -1 ) and to_type =? `, ty).
		OrderBy(`LENGTH("value"), "value"`).
		Limit(100).
		Find(&orders)

	return orders, err
}

func (monitor *Monitor) updateSendOrderOutTxStatus(tx string) error {
	order := &SendOrder{Status: 1}

	update, err := monitor.tokenswapdb.Where(`status = 2 and out_tx = ?`, tx).Cols("status").Update(order)

	if err != nil {
		monitor.ErrorF("update send orders out_tx status error :%s,out_tx:%s", err.Error(), tx)
		return err
	}

	monitor.InfoF("update send orders out_tx status out_tx: %s update:%d", tx, update)

	return nil
}

func (monitor *Monitor) addSendOrderOutTx(id int64, tx string, status int64, retry int32) error {
	order := &SendOrder{ID: id, OutTx: tx, Status: status, Retry: retry}

	update, err := monitor.tokenswapdb.Where(`(status = 0 or status = -1) and i_d = ?`, id).Cols("out_tx", "status", "retry").Update(order)

	if err != nil {
		monitor.ErrorF("add send orders out_tx error :%s ,tx:%s", err.Error(), tx)
	}

	monitor.InfoF("add send orders out_tx : %s update:%d ", tx, update)

	return err
}

func (monitor *Monitor) updateEthSendOrderRetry(id int64, retry int32) error {
	order := &SendOrder{ID: id, Retry: retry, Status: 0}

	update, err := monitor.tokenswapdb.Where(`status = -1 and i_d = ?`, id).
		Cols("retry", "status").Update(order)

	if err != nil {
		monitor.ErrorF("update send orders retry error :%s", err.Error())
	}

	monitor.InfoF("update send orders id:%d, retry: %d update:%d ", order.ID, order.Retry, update)

	return err
}

func (monitor *Monitor) NeoSendMoniter() {
	tick := time.NewTicker(time.Second * 30)

	for {
		select {
		case <-tick.C:

			sendOrders, err := monitor.getSendOrder(2)

			if err != nil {

				monitor.ErrorF("query send orders error :%s", err.Error())

			} else {

				var tokenBalance uint64
				var err error

				if len(sendOrders) > 0 {
					from := ToInvocationAddress(monitor.NEOKeyAddress)
					tokenBalance, err = monitor.neoClient.Nep5BalanceOf(monitor.tncOfNEO, from)
					if err != nil {
						monitor.ErrorF("get neo(%s) balance err: %s ", monitor.NEOKeyAddress, err.Error())
						continue
					}
					monitor.InfoF("NEO-TNC wallet(%s)  balance :%d !", monitor.NEOKeyAddress, tokenBalance)

					for _, v := range sendOrders {
						amount, b := ethmath.ParseUint64(v.Value)
						if !b {
							monitor.ErrorF("send neo ParseUint64  %s  err  ", v.Value)
							continue
						}

						if tokenBalance < uint64(amount) {
							monitor.ErrorF("NEO-TNC wallet(%s)  balance is  not enough !", monitor.NEOKeyAddress)
							break
						}

						order := new(Order)
						order.Value = v.Value
						order.To = v.To
						order.TX = v.OrderTx

						tx, err := monitor.sendNEO(order)
						if err != nil {
							monitor.ErrorF(" send NEO error :%s To:%s, value:%s", err.Error(), v.To, v.Value)
							continue
						}

						monitor.addSendOrderOutTx(v.ID, tx, 2, order.Retry)

						err = monitor.insertLogAndUpdate(nil, order, "tax_cost", "send_value")
						if err != nil {
							monitor.ErrorF(" update send NEO log error :%s ", err.Error())
						}

						tokenBalance -= amount
					}
				}
			}
		}
	}
}

func (monitor *Monitor) EthSendMoniter() {
	tick := time.NewTicker(time.Second * 30)

	for {
		select {
		case <-tick.C:

			sendOrders, err := monitor.getSendOrder(1)

			if err != nil {

				monitor.ErrorF("query send orders error :%s", err.Error())

			} else {
				var balance *big.Int
				var err error

				if len(sendOrders) > 0 {
					balance, err = monitor.ethClient.GetTokenBalance(monitor.tncOfETH, monitor.ETHKeyAddress)
					if err != nil {
						monitor.ErrorF("get eth(%s) balance err: %s ", monitor.ETHKeyAddress, err.Error())
						continue
					}

					monitor.InfoF("ETH-TNC wallet(%s)  balance :%s !", monitor.ETHKeyAddress, balance.String())

					for _, v := range sendOrders {
						amount, b := ethmath.ParseUint64(v.Value)
						if !b {
							monitor.ErrorF("send eth ParseUint64  %s  err  ", v.Value)
							continue
						}

						bigAmount := big.NewInt(int64(amount))

						if balance.Cmp(bigAmount) < 0 {
							monitor.ErrorF("ETH-TNC wallet(%s)  balance is  not enough !", monitor.ETHKeyAddress)
							break
						}

						order := new(Order)
						order.Value = v.Value
						order.To = v.To
						order.TX = v.OrderTx
						order.Retry = v.Retry

						sendSuccess := false
						for {

							tx, err := monitor.sendETH(order)
							if err != nil {
								monitor.ErrorF(" send ETH error :%s To:%s, value:%s", err.Error(), v.To, v.Value)
								break
							}

							err = monitor.insertLogAndUpdate(nil, order, "tax_cost", "send_value")
							if err != nil {
								monitor.ErrorF(" update send NEO log error :%s ", err.Error())
							}

							order.Retry++

							// pending中
							monitor.addSendOrderOutTx(v.ID, tx, -1, order.Retry)

							if !monitor.waitEthTx(tx, v.ID) {
								monitor.DebugF("waitEthTx pending time out :%x", tx)
								continue
							}

							// 发送中
							monitor.addSendOrderOutTx(v.ID, tx, 2, order.Retry)

							sendSuccess = true
							break

						}

						if sendSuccess {
							balance.Sub(balance, bigAmount)
						}
					}
				}
			}
		}
	}
}

func (monitor *Monitor) waitEthTx(tx string, id int64) bool {
	timeOut := time.After(time.Minute * 30)

	monitor.DebugF("waitEthTx:%s", tx)

	tick := time.NewTicker(time.Second * 60)

	for {
		select {
		case inTx := <-ethTxChan:
			if inTx == tx {
				return true
			}
		case <-tick.C:
			tx, err := monitor.ethClient.GetTransactionByHash(tx)

			// 查不到tx
			if err == nil && tx == nil {
				monitor.ErrorF("tx was replaced :%s", tx)

				monitor.updateEthSendOrderRetry(id, 0)

				return true
			}
		case <-timeOut:
			return false
		}
	}
}

func (monitor *Monitor) CheckEthBlockNumber(tx string, needNumber uint64) bool {
	tick := time.NewTicker(time.Second * time.Duration(monitor.ethGetBlockInterval))
	for {
		select {
		case <-tick.C:
			cursorBlock, err := monitor.ethClient.BlockNumber()
			if err != nil {
				monitor.ErrorF("eth BlockNumber err:%s", err.Error())
				continue
			}

			if cursorBlock >= needNumber+uint64(monitor.ethConfirmCount) {

				tx, err := monitor.ethClient.GetTransactionByHash(tx)

				// 确认后，查不到tx，交易失败
				if err == nil && tx == nil {
					monitor.ErrorF("tx was replaced :%s", tx)

					return false
				}

				return true
			}

			monitor.DebugF("CheckEthBlockNumber %d - %d ", cursorBlock, needNumber)
		}
	}
}

func (monitor *Monitor) CheckNeoBlockNumber(needNumber uint64) {
	tick := time.NewTicker(time.Second * time.Duration(monitor.neoGetBlockInterval))
	for {
		select {
		case <-tick.C:
			cursorBlock, err := monitor.neoClient.GetBlockCount()
			if err != nil {
				monitor.ErrorF("eth BlockNumber err:%s", err.Error())
				continue
			}

			if uint64(cursorBlock) >= needNumber+uint64(monitor.neoConfirmCount) {
				return
			}

			monitor.DebugF("CheckNeoBlockNumber %d - %d ", cursorBlock, needNumber)
		}
	}
}

// ToInvocationAddress neo wallet address to invocation address
func ToInvocationAddress(address string) string {
	bytesOfAddress, _ := decodeAddress(address)

	return hex.EncodeToString(reverseBytes(bytesOfAddress))
}

func reverseBytes(s []byte) []byte {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}

	return s
}

func decodeAddress(address string) ([]byte, error) {

	result, _, err := base58.CheckDecode(address)

	if err != nil {
		return nil, err
	}

	return result[0:20], nil
}
