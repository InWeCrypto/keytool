# Inwecrypto Keytool应用

这一工具用于从inwecrypto的keystore/mnemonic中获取私钥。

这一工具是桌面应用，不用联网。为了安全起见，可以在使用这一工具时关闭您的网络链接。

我们提供Windows和Mac系统的桌面应用，也提供了Linux系统的命令行。

您可以从这个地址中获取压缩包： https://github.com/InWeCrypto/keytool/releases 

>mac-gui-amd64.zip ---- mac app

>windows-gui-386.zip ---- win32 app

>windows-gui-amd64.zip ---- win64 app

>prkeycli_linux_64.zip  ---- linux command line 

>prkeycli_mac.zip ---- mac command line 

>prkeycli_win_64.exe.zip ---- win64 command line

如果您有兴趣搭建项目，可以通过这里用golang语言搭建界面： https://github.com/asticode/go-astilectron

使用命令行:
1. 解压缩cli tool到工作目录.

2. 显示工具帮助:./prkey_mac -h Usage of ./prkey_mac: -keystore string Keystore file path -lang string Mnemonic language en_US or zh_CN (default "en_US") -mnemonic string Mnemonic string -password string Keystore password 

3. keystore

- 获取keystore信息，写如文件 eg. mykey.json
- ./prkey_mac -keystore < keystore file path, eg. mykey.json > -password < keystore password>

4. 助记词
- ./prkey_mac -mnemonic "your mnemonic string" -lang "en_US" ##if your mnemonic is English
- ./prkey_mac -mnemonic "your mnemonic string" -lang "zh_CN" ## if your mnemonic is Chinese

应用使用方法：

1. 解压缩文件，不需要安装，双击文件即可。
![](https://raw.githubusercontent.com/InWeCrypto/keytool/master/mac_security.jpg)
2. Mac系统中可能需要您在设置中找到安全与隐私，开启应用。
3. Windows系统可能需要您检查防毒软件，确保app可以打开。
4.
![](https://github.com/biubiubird/keytool/blob/master/resources/4-cn.jpg?raw=true)
 
