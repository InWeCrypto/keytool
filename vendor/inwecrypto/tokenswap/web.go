package tokenswap

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/dynamicgo/config"
	"github.com/dynamicgo/slf4go"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-xorm/xorm"
	ethrpc "github.com/inwecrypto/ethgo/rpc"
	neorpc "github.com/inwecrypto/neogo/rpc"
	neotx "github.com/inwecrypto/neogo/tx"
)

type Response struct {
	Code  int
	Error string
	Data  interface{}
}

type WebServer struct {
	slf4go.Logger
	engine          *gin.Engine
	db              *xorm.Engine
	laddr           string
	TXGenerate      *snowflake.Node
	keyAddressOfETH string
	keyAddressOFNEO string
	limitAmount     int64  // 最低转账数量
	maxAmount       int64  // 最大转账数量
	neo2ethtax      string // 转账费率
	eth2neotax      string
	ethClient       *ethrpc.Client
	neoClient       *neorpc.Client
	tncOfNEO        string
	tncOfETH        string
}

func LimitMiddleware(limit int64, interval int64) gin.HandlerFunc {
	tick := time.Tick(time.Duration(interval) * time.Second)
	count := int64(0)

	return func(c *gin.Context) {
		select {
		case <-tick:
			count = 0
		default:
			count++
			if count < limit {
				c.Next()
			} else {
				c.JSON(http.StatusForbidden, Response{1, "request too frequently, server is busy.", nil})
				c.Abort()
			}
		}
	}
}

func NewWebServer(conf *config.Config) (*WebServer, error) {
	tokenswapdb, err := createEngine(conf, "tokenswapdb")

	if err != nil {
		return nil, fmt.Errorf("create tokenswap db engine error %s", err)
	}

	limit := conf.GetInt64("tokenswap.requestlimit", 1000)
	interval := conf.GetInt64("tokenswap.requestinterval", 10)

	engine := gin.New()

	engine.Use(LimitMiddleware(limit, interval))
	engine.Use(gin.Recovery())

	ethKey, err := readETHKeyStore(conf, "eth.keystore", conf.GetString("eth.keystorepassword", ""))

	if err != nil {
		return nil, fmt.Errorf("create neo db engine error %s", err)
	}

	neoKey, err := readNEOKeyStore(conf, "neo.keystore", conf.GetString("neo.keystorepassword", ""))

	if err != nil {
		return nil, fmt.Errorf("create neo db engine error %s", err)
	}

	node, err := snowflake.NewNode(1)
	if err != nil {
		return nil, err
	}

	server := &WebServer{
		engine:          engine,
		Logger:          slf4go.Get("tokenswap-server"),
		laddr:           conf.GetString("tokenswap.webladdr", ":8000"),
		db:              tokenswapdb,
		TXGenerate:      node,
		keyAddressOfETH: ethKey.Address,
		keyAddressOFNEO: neoKey.Address,
		limitAmount:     conf.GetInt64("tokenswap.limitamount", 10000),
		maxAmount:       conf.GetInt64("tokenswap.maxamount", 100000),
		neo2ethtax:      conf.GetString("tokenswap.neo2ethtax", "0.001"),
		eth2neotax:      conf.GetString("tokenswap.eth2neotax", "0.001"),
		ethClient:       ethrpc.NewClient(conf.GetString("eth.node", "")),
		neoClient:       neorpc.NewClient(conf.GetString("neo.node", "")),
		tncOfETH:        conf.GetString("eth.tnc", ""),
		tncOfNEO:        conf.GetString("neo.tnc", ""),
	}

	// gin log write to backend
	server.engine.Use(gin.LoggerWithWriter(server))

	server.makeRouters()

	return server, nil
}

// implement of io.writer
func (server *WebServer) Write(p []byte) (n int, err error) {
	server.Info(string(p))
	return len(p), nil
}

// Run run http service
func (server *WebServer) Run() error {

	// 允许跨域
	config := cors.DefaultConfig()
	config.AllowHeaders = append(config.AllowHeaders, "Accept")
	config.AllowHeaders = append(config.AllowHeaders, "Authorization")
	config.AllowHeaders = append(config.AllowHeaders, "X-Requested-With")
	config.AllowHeaders = append(config.AllowHeaders, "ct")
	config.AllowHeaders = append(config.AllowHeaders, "Lang")
	config.AllowAllOrigins = true
	server.engine.Use(cors.New(config))

	return server.engine.Run(server.laddr)
}

func (server *WebServer) makeRouters() {
	server.engine.POST("/trade", server.CreateOrder)
	server.engine.GET("/trade/:tx", server.GetOrder)
	server.engine.GET("/log/:tx", server.GetOrderLog)
	server.engine.GET("/tradeinfo", server.TradeInfo)
}

func (server *WebServer) TradeInfo(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("access-control-allow-headers", "Content-Type, Accept, Authorization, X-Requested-With, ct, Origin, X_Requested_With, Lang")

	info := make(map[string]interface{})
	info["limitAmount"] = server.limitAmount
	info["maxAmount"] = server.maxAmount
	info["eth2neotax"] = server.eth2neotax
	info["neo2ethtax"] = server.neo2ethtax

	ctx.JSON(http.StatusOK, Response{0, "", info})
}

func (server *WebServer) GetOrderLog(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("access-control-allow-headers", "Content-Type, Accept, Authorization, X-Requested-With, ct, Origin, X_Requested_With, Lang")

	tx := ctx.Param("tx")
	logs := make([]Log, 0)
	//	err := server.db.Where(` "t_x" = ?`, tx).Desc("create_time").Find(&logs)
	err := server.db.Where(` "t_x" = ?`, tx).OrderBy("create_time").Find(&logs)
	if err != nil {
		ctx.JSON(http.StatusOK, Response{1, err.Error(), nil})
		return
	}

	ctx.JSON(http.StatusOK, Response{0, "", logs})
}

func (server *WebServer) GetOrder(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("access-control-allow-headers", "Content-Type, Accept, Authorization, X-Requested-With, ct, Origin, X_Requested_With, Lang")

	tx := ctx.Param("tx")
	order := &Order{}
	_, err := server.db.Where(` "t_x" = ?`, tx).Get(order)
	if err != nil {
		ctx.JSON(http.StatusOK, Response{1, err.Error(), nil})
		return
	}
	ctx.JSON(http.StatusOK, Response{0, "", order})
}

func (server *WebServer) checkNEOBalance(transferValue int64) error {
	from := ToInvocationAddress(server.keyAddressOFNEO)
	tokenBalance, err := server.neoClient.Nep5BalanceOf(server.tncOfNEO, from)
	if err != nil {
		server.ErrorF("get neo(%s) balance err: %v ", server.keyAddressOFNEO, err)
		return errors.New("trader NEO-TNC account balance not enough")
	}

	if tokenBalance < uint64(transferValue) {
		server.ErrorF("neo(%s) balance:%d , transferValue:%d, balance not enough  err: %v ",
			server.keyAddressOFNEO, tokenBalance, transferValue, err)
		return errors.New("trader NEO-TNC account balance not enough")
	}

	return nil
}

func (server *WebServer) checkETHBalance(transferValue int64) error {
	balance, err := server.ethClient.GetTokenBalance(server.tncOfETH, server.keyAddressOfETH)
	if err != nil {
		server.ErrorF("get eth(%s) balance err: %v ", server.keyAddressOfETH, err)
		return errors.New("trader ETH-TNC account balance not enough")
	}

	if balance.Int64() < transferValue {
		server.ErrorF("eth(%s) balance:%d , transferValue:%d, balance not enough  err: %v ",
			server.keyAddressOfETH, balance.Int64(), transferValue, err)
		return errors.New("trader ETH-TNC account balance not enough")
	}

	return nil
}

func (server *WebServer) CreateOrder(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("access-control-allow-headers", "Content-Type, Accept, Authorization, X-Requested-With, ct, Origin, X_Requested_With, Lang")

	from := ctx.Query("from")
	to := ctx.Query("to")
	value := ctx.Query("value")

	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		ctx.JSON(http.StatusOK, Response{1, err.Error(), nil})
		return
	}

	if amount > float64(server.maxAmount) {
		ctx.JSON(http.StatusOK, Response{1, "amount too large, max amount: " + fmt.Sprint(server.maxAmount), nil})
		return
	}

	if amount < float64(server.limitAmount) {
		ctx.JSON(http.StatusOK, Response{1, "amount must over " + fmt.Sprint(server.limitAmount), nil})
		return
	}

	// TODO 参数校验
	if from == "" || to == "" || amount <= 0 {
		ctx.JSON(http.StatusOK, Response{1, "param error", nil})
		return
	}

	// 地址长度格式校验
	if strings.Index(from, "0x") >= 0 && len(from) == 42 {
		from = strings.ToLower(from)

		if len(to) != 34 {
			ctx.JSON(http.StatusOK, Response{1, "address error", nil})
			return
		}

	} else {
		if strings.Index(to, "0x") >= 0 && len(to) == 42 {
			to = strings.ToLower(to)

			if len(from) != 34 {
				ctx.JSON(http.StatusOK, Response{1, "address error", nil})
				return
			}
		} else {
			ctx.JSON(http.StatusOK, Response{1, "address error", nil})
			return
		}
	}

	res := make(map[string]string)

	// 添加随机数,防止重放
	r := rand.Intn(9999) + 1
	fx8value := neotx.MakeFixed8(float64(amount) + float64(r)/float64(10000))

	if fx8value < 0 {
		ctx.JSON(http.StatusOK, Response{1, "amount param error", nil})
		return
	}

	if strings.Index(from, "0x") >= 0 && len(from) == 42 {
		res["Address"] = server.keyAddressOfETH

		//		err := server.checkNEOBalance(int64(fx8value))
		//		if err != nil {
		//			ctx.JSON(http.StatusOK, Response{1, err.Error(), nil})
		//			return
		//		}
	}

	if strings.Index(to, "0x") >= 0 && len(to) == 42 {
		res["Address"] = server.keyAddressOFNEO

		//		err := server.checkETHBalance(int64(fx8value))
		//		if err != nil {
		//			ctx.JSON(http.StatusOK, Response{1, err.Error(), nil})
		//			return
		//		}
	}

	res["Value"] = fx8value.String()

	order := Order{
		TX:         server.TXGenerate.Generate().String(),
		From:       from,
		To:         to,
		Value:      fmt.Sprint(int64(fx8value)),
		CreateTime: time.Now(),
	}

	_, err = server.db.Insert(order)
	if err != nil {
		ctx.JSON(http.StatusOK, Response{1, err.Error(), nil})
		return
	}

	res["TX"] = order.TX
	res["CreateTime"] = fmt.Sprint(order.CreateTime.Unix())

	ctx.JSON(http.StatusOK, Response{0, "", res})
}
