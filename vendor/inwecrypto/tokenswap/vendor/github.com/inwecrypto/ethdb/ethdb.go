package ethdb

import "time"

// TableTx orm table define
type TableTx struct {
	ID         int64     `xorm:"pk autoincr"`
	TX         string    `xorm:"index notnull"`
	From       string    `xorm:"index(from_to)"`
	To         string    `xorm:"index(from_to)"`
	Asset      string    `xorm:"notnull"`
	Value      string    `xorm:"notnull"`
	Blocks     uint64    `xorm:"notnull index"`
	GasPrice   string    `xorm:"notnull"`
	Gas        string    `xorm:"notnull"`
	CreateTime time.Time `xorm:"TIMESTAMP notnull"`
}

// TableName xorm table name
func (table *TableTx) TableName() string {
	return "eth_tx"
}

// TableOrder .
type TableOrder struct {
	ID          int64      `json:"-" xorm:"pk autoincr"`
	TX          string     `json:"tx" xorm:"index notnull"`
	From        string     `json:"from" xorm:"index(from_to)"`
	To          string     `json:"to" xorm:"index(from_to)"`
	Asset       string     `json:"asset" xorm:"notnull"`
	Value       string     `json:"value" xorm:"notnull"`
	Blocks      int64      `json:"blocks" xorm:"default (-1)"`
	CreateTime  time.Time  `json:"createTime,omitempty" xorm:"TIMESTAMP notnull created"`
	ConfirmTime *time.Time `json:"confirmTime,omitempty" xorm:"TIMESTAMP"`
	Context     *string    `json:"context" xorm:"TEXT"`
}

// TableName xorm table name
func (table *TableOrder) TableName() string {
	return "eth_order"
}

// TableWallet .
type TableWallet struct {
	ID         int64     `xorm:"pk autoincr"`
	Address    string    `xorm:"index(address_userid)"`
	UserID     string    `xorm:"index(address_userid)"`
	CreateTime time.Time `xorm:"TIMESTAMP notnull created"`
}

// TableName xorm table name
func (table *TableWallet) TableName() string {
	return "eth_wallet"
}
