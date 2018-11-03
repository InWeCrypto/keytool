##  tokenswap web api 说明

### 创建订单
POST /trade?from=xxxx&to=xxxx&value=xxx
```
参数说明
from  string     转账发起地址
to    string     收账地址
value int 		 数量


成功 http status 200 : 返回json对象
失败 http status 4xx : 错误原因说明

eg:
请求 : http://127.0.0.1:8001/trade?from=ANyhCjypLxJTH6AsXojafVxUycfPvCdqeW&to=0x83226e522a25da60bea24ab4d16f159958d3567c&value=1032

返回 : json对象

{
"TX": "976420902604378112",
"Value": "1032.9672",
"Address": "Ab8vffxvjaA3JKm3weBg6ChmZMSvorMoBM"
}

TX   	订单号
Value	加了随机数后的值
Address 付账地址 
```

### 获取订单信息
GET /trade/:tx

```
参数说明
tx   string  订单的txId号

成功 http status 200 : 返回该订单的详情
失败 http status 4xx : 错误原因说明

eg:
请求 : http://127.0.0.1:8001/trade/976396860329562112 
返回 : json对象
{
"ID": 3,
"TX": "976396860329562112",
"From": "ANyhCjypLxJTH6AsXojafVxUycfPvCdqeW",
"To": "0x83226e522a25da60bea24ab4d16f159958d3567c",
"Value": "5",
"InTx": "",
"OutTx": "",
"CreateTime": "2018-03-21T17:55:26+08:00",
"CompletedTime": "0001-01-01T00:00:00Z"
}

返回参数说明 
TX 订单号
From   转账发起地址
To 	   转账到达地址
Value  转账数量
Intx   发起链的交易单号
OutTx  收账链的交易单号
```

### 获取订单处理状态
GET /log/:tx
```
参数说明
tx   string  订单的txId号

成功 http status 200 : 返回该订单的处理流程
失败 http status 4xx : 错误原因说明

eg:
请求 : http://127.0.0.1:8001/log/976396860329562112
返回 : json 数组
[
{
"TX": "976396860329562112",
"CreateTime": "2018-03-21T18:13:47+08:00",
"Content": "3"
},
{
"TX": "976396860329562112",
"CreateTime": "2018-03-21T18:13:39+08:00",
"Content": "2"
},
{
"TX": "976396860329562112",
"CreateTime": "2018-03-21T18:13:30+08:00",
"Content": "11"
}
]

Content   当前处理步骤说明
```