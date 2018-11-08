# Inwecrypto keytool

This tool is to get the private key from the keystore or mnemonic for inwecrypto.

the tool is an desktop app, it does not need to connect the internet， for your own security you can shutdown your network when using the tool.

we provide windows and mac desktop apps, but we aslo provide the command line tools for linxu windows and mac.

you can get the compiled file from https://github.com/InWeCrypto/keytool/releases

```
mac-gui-amd64.zip ---- mac app
windows-gui-386.zip ---- win32 app
windows-gui-amd64.zip ---- win64 app

prkeycli_linux_64.zip  ---- linux command line
prkeycli_mac.zip ---- mac command line
prkeycli_win_64.exe.zip ---- win64 command line
```

if you are interesing in compile the project, you can refer to the https://github.com/asticode/go-astilectron, which is a gui project for golang



command line usage:

1) unzip the cli tool into your work directory.
2) show the command help:
./prkey_mac -h
Usage of ./prkey_mac:
  -keystore string
    	Keystore file path
  -lang string
    	Mnemonic language en_US or zh_CN (default "en_US")
  -mnemonic string
    	Mnemonic string
  -password string
    	Keystore password

3) for keystore
    * get the keystore information, write it to the file, eg. mykey.json
    * ./prkey_mac -keystore < keystore file path, eg. mykey.json > -password < keystore password>

4) for mnemonic
    * ./prkey_mac -mnemonic "your mnemonic string" -lang "en_US"  ##if your mnemonic is English
    * ./prkey_mac -mnemonic "your mnemonic string" -lang "zh_CN"  ## if your mnemonic is Chinese

app usage:
1) upzip the app file, it does not need to install, just double-click to run the file.
2) for mac maybe you should set the security setting.
![mac setting](https://github.com/InWeCrypto/keytool/blob/master/mac_security.jpg?raw=true)
3) for windows maybe you should check if the anti-virus software， making sure it will not stop app.
4)
![app usage](https://github.com/InWeCrypto/keytool/blob/master/app_usage.jpg?raw=true)
