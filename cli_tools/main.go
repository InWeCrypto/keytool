package main

import (
	"flag"
	"io/ioutil"
	"os"

	"github.com/inwecrypto/mobilesdk/neomobile"
)

func init() {
	flag.Parse()
}

var keystore = flag.String("keystore", "", "Keystore file path")
var psword = flag.String("password", "", "Keystore password")
var mnemonic = flag.String("mnemonic", "", "Mnemonic string")
var lang = flag.String("lang", "en_US", "Mnemonic language en_US or zh_CN")

func main() {
	if (*keystore != "") && (*psword != "") {
		fi, err := os.Open(*keystore)
		if err != nil {
			panic(err)
		}
		defer fi.Close()
		keystring, err := ioutil.ReadAll(fi)
		if err != nil {
			panic(err)
		}

		prkey, err := neomobile.FromKeyStore(string(keystring), *psword)

		if err != nil {
			println(err)
		}
		println("\n\n private key: " + prkey)
	} else if (*mnemonic != "") && (*lang != "") {
		prkey, err := neomobile.FromMnemonic(*mnemonic, *lang)

		if err != nil {
			println(err)
		}
		println("\n\n private key: " + prkey)
	} else {
		println("please use the right options")
	}

}
