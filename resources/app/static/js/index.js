let index = {
    about: function(html) {
        let c = document.createElement("div");
        c.innerHTML = html;
        asticode.modaler.setContent(c);
        asticode.modaler.show();
    },
    fromKeystore() {
        let ksubmit = document.getElementById("ksubmit");
        ksubmit.onclick = function() {
            let keystore = document.getElementById("ks").value;
            let password = document.getElementById("pd").value;
            let message = {"name":"fromkeystore"};
            let payload = [keystore,password];
            message.payload = payload;
            console.log(message)
            index.explore(message); 
        };
    
    },
    fromMnemonic() {
        let mcsubmit = document.getElementById("mcsubmit");
        mcsubmit.onclick = function() {
            let mnemonic = document.getElementById("mc").value;
            let lang=""
            var obj = document.getElementsByName("lang");
            for(var i=0; i<obj.length; i ++){
                if(obj[i].checked){
                    lang = obj[i].value;
                    };
                };
            let message = {"name":"frommnemonic"};
            let payload = [mnemonic,lang];
            message.payload = payload;
            index.explore(message); 
            };
        
        },
    init: function() {
        // Init
        asticode.loader.init();
        asticode.modaler.init();
        asticode.notifier.init();

        // Wait for astilectron to be ready
        document.addEventListener('astilectron-ready', function() {
            // Listen
            
            index.listen();
            
            // Explore default path
            index.fromKeystore();
            index.fromMnemonic();
            
        })
    },
    explore: function(message) {
        asticode.loader.show();
        astilectron.sendMessage(message, function(message) {
            // Init
            asticode.loader.hide();

            // Check error
            if (message.name === "error") {
                asticode.notifier.error(message.payload);
                return
            }
            // Process path
            document.getElementById("pk").innerHTML = message.payload;
        })
    },
    listen: function() {
        astilectron.onMessage(function(message) {
            switch (message.name) {
                case "about":
                    index.about(message.payload);
                    return {payload: "payload"};
                    break;
                case "check.out.menu":
                    asticode.notifier.info(message.payload);
                    break;
            }
        });
    }
};