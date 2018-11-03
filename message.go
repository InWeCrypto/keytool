package main

import (
	"encoding/json"

	"github.com/asticode/go-astilectron"
	"github.com/asticode/go-astilectron-bootstrap"

	"github.com/inwecrypto/mobilesdk/neomobile"
)

// handleMessages handles messages
func handleMessages(_ *astilectron.Window, m bootstrap.MessageIn) (payload interface{}, err error) {

	switch m.Name {
	case "fromkeystore":
        var ks [] string
		if len(m.Payload) > 0 {
			if err = json.Unmarshal(m.Payload, &ks); err != nil {
				payload = err.Error()
				return
			}
		}
		if payload, err = neomobile.FromKeyStore(ks[0], ks[1]); err != nil {
			payload = err.Error()
			return
		}
	case "frommnemonic":
        var mc [] string
		if len(m.Payload) > 0 {
			// Unmarshal payload
			if err = json.Unmarshal(m.Payload, &mc); err != nil {
				payload = err.Error()
				return
			}
		}

		if payload, err = neomobile.FromMnemonic(mc[0], mc[1]); err != nil {
			payload = err.Error()
			return
		}
	return
}
return 
}