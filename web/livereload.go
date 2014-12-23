/* ----------------------------------------------------------------------
 *       ______      ___                         __
 *      / ____/___  /   |  ____  __  ___      __/ /_  ___  ________
 *     / / __/ __ \/ /| | / __ \/ / / / | /| / / __ \/ _ \/ ___/ _ \
 *    / /_/ / /_/ / ___ |/ / / / /_/ /| |/ |/ / / / /  __/ /  /  __/
 *    \____/\____/_/  |_/_/ /_/\__. / |__/|__/_/ /_/\___/_/   \___/
 *                            /____/
 *
 * (C) Copyright 2014 GoAnywhere (http://goanywhere.io).
 * ----------------------------------------------------------------------
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 * ----------------------------------------------------------------------*/
package web

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type (
	lrserver struct {
		tunnels    map[*tunnel]bool
		broadcast  chan []byte
		javascript []byte

		in  chan *tunnel
		out chan *tunnel

		mutex sync.RWMutex
	}

	tunnel struct {
		ws      *websocket.Conn
		message chan []byte
	}

	hello struct {
		Command    string   `json:"command"`
		Protocols  []string `json:"protocols"`
		ServerName string   `json:"serverName"`
	}

	alert struct {
		Command string `json:"command"`
		Message string `json:"message"`
	}

	reload struct {
		Command string `json:"command"`
		Path    string `json:"path"`    // as full as possible/known, absolute path preferred, file name only is OK
		LiveCSS bool   `json:"liveCSS"` // false to disable live CSS refresh
	}
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

/* ----------------------------------------------------------------------
 * WebSocket Server
 * ----------------------------------------------------------------------*/
// Alert sends a notice message to browser's livereload.js.
func (self *lrserver) Alert(message string) {
	go func() {
		var bytes, _ = json.Marshal(&alert{
			Command: "alert",
			Message: message,
		})
		self.broadcast <- bytes
	}()
}

// Reload sends a reload message to browser's livereload.js.
func (self *lrserver) Reload(path string) {
	go func() {
		var bytes, _ = json.Marshal(&reload{
			Command: "reload",
			Path:    path,
			LiveCSS: true,
		})
		self.broadcast <- bytes
	}()
}

// Serve serves as a livereload server for accepting I/O tunnel messages.
func (self *lrserver) Serve(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	tn := new(tunnel)
	tn.ws = ws
	tn.message = make(chan []byte, 256)
	self.in <- tn
	defer func() { self.out <- tn }()
	tn.connect()
}

// ServeJS serves livereload.js for browser.
func (self *lrserver) ServeJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write(self.javascript)
}

// Start activates livereload server for accepting tunnel messages.
func (self *lrserver) Start() {
	go func() {
		for {
			select {
			case tunnel := <-self.in:
				self.mutex.Lock()
				defer self.mutex.Unlock()
				self.tunnels[tunnel] = true

			case tunnel := <-self.out:
				self.mutex.Lock()
				defer self.mutex.Unlock()
				delete(self.tunnels, tunnel)
				close(tunnel.message)

			case m := <-self.broadcast:
				for tunnel := range self.tunnels {
					select {
					case tunnel.message <- m:
					default:
						self.mutex.Lock()
						defer self.mutex.Unlock()
						delete(self.tunnels, tunnel)
						close(tunnel.message)
					}
				}
			}
		}
	}()
}

/* ----------------------------------------------------------------------
 * WebSocket Server Tunnel
 * ----------------------------------------------------------------------*/
// connect reads/writes message for livereload.js.
func (self *tunnel) connect() {
	// ***********************
	// WebSocket Tunnel#Write
	// ***********************
	go func() {
		for message := range self.message {
			if err := self.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				break
			} else {
				log.Printf("[WebSocket][Write] %s", message)
				if bytes.Contains(message, []byte(`"command":"hello"`)) {
					log.Printf("[WebSocket] connection established")
					Livereload.Alert("Connected")
				}
			}
		}
		self.ws.Close()
	}()
	// ***********************
	// WebSocket Tunnel#Read
	// ***********************
	for {
		_, message, err := self.ws.ReadMessage()
		if err != nil {
			break
		}
		switch true {
		case bytes.Contains(message, []byte(`"command":"hello"`)):
			var bytes, _ = json.Marshal(&hello{
				Command:    "hello",
				Protocols:  []string{"http://livereload.com/protocols/official-7"},
				ServerName: "Rex#Livereload",
			})
			self.message <- bytes
		}
	}
	self.ws.Close()
}

/* ----------------------------------------------------------------------
 * WebSocket Livereload
 * ----------------------------------------------------------------------*/
var Livereload = lrserver{
	broadcast: make(chan []byte),
	in:        make(chan *tunnel),
	out:       make(chan *tunnel),
	tunnels:   make(map[*tunnel]bool),
	// SEE https://github.com/livereload/livereload-js
	//     http://feedback.livereload.com/knowledgebase/articles/86174-livereload-protocol
	// 2014-05-03 e6b5ac4@jscompress.com
	javascript: []byte(`(function(){var e={},t={},n={},r={},i={},s={},o={},u={},a={};var f,l,c,h,p=[].indexOf||function(e){for(var t=0,n=this.length;t<n;t++){if(t in this&&this[t]===e)return t}return-1};e.PROTOCOL_6=f="http://livereload.com/protocols/official-6";e.PROTOCOL_7=l="http://livereload.com/protocols/official-7";e.ProtocolError=h=function(){function e(e,t){this.message="LiveReload protocol error ("+e+') after receiving data: "'+t+'".'}return e}();e.Parser=c=function(){function e(e){this.handlers=e;this.reset()}e.prototype.reset=function(){return this.protocol=null};e.prototype.process=function(e){var t,n,r,i,s;try{if(this.protocol==null){if(e.match(/^!!ver:([\d.]+)$/)){this.protocol=6}else if(r=this._parseMessage(e,["hello"])){if(!r.protocols.length){throw new h("no protocols specified in handshake message")}else if(p.call(r.protocols,l)>=0){this.protocol=7}else if(p.call(r.protocols,f)>=0){this.protocol=6}else{throw new h("no supported protocols found")}}return this.handlers.connected(this.protocol)}else if(this.protocol===6){r=JSON.parse(e);if(!r.length){throw new h("protocol 6 messages must be arrays")}t=r[0],i=r[1];if(t!=="refresh"){throw new h("unknown protocol 6 command")}return this.handlers.message({command:"reload",path:i.path,liveCSS:(s=i.apply_css_live)!=null?s:true})}else{r=this._parseMessage(e,["reload","alert"]);return this.handlers.message(r)}}catch(o){n=o;if(n instanceof h){return this.handlers.error(n)}else{throw n}}};e.prototype._parseMessage=function(e,t){var n,r,i;try{r=JSON.parse(e)}catch(s){n=s;throw new h("unparsable JSON",e)}if(!r.command){throw new h('missing "command" key',e)}if(i=r.command,p.call(t,i)<0){throw new h("invalid command '"+r.command+"', only valid commands are: "+t.join(", ")+")",e)}return r};return e}();var d,f,l,c,v,m;m=e,c=m.Parser,f=m.PROTOCOL_6,l=m.PROTOCOL_7;v="2.0.8";t.Connector=d=function(){function e(e,t,n,r){var i=this;this.options=e;this.WebSocket=t;this.Timer=n;this.handlers=r;this._uri="ws://"+this.options.host+":"+this.options.port+"/livereload";this._nextDelay=this.options.mindelay;this._connectionDesired=false;this.protocol=0;this.protocolParser=new c({connected:function(e){i.protocol=e;i._handshakeTimeout.stop();i._nextDelay=i.options.mindelay;i._disconnectionReason="broken";return i.handlers.connected(e)},error:function(e){i.handlers.error(e);return i._closeOnError()},message:function(e){return i.handlers.message(e)}});this._handshakeTimeout=new n(function(){if(!i._isSocketConnected()){return}i._disconnectionReason="handshake-timeout";return i.socket.close()});this._reconnectTimer=new n(function(){if(!i._connectionDesired){return}return i.connect()});this.connect()}e.prototype._isSocketConnected=function(){return this.socket&&this.socket.readyState===this.WebSocket.OPEN};e.prototype.connect=function(){var e=this;this._connectionDesired=true;if(this._isSocketConnected()){return}this._reconnectTimer.stop();this._disconnectionReason="cannot-connect";this.protocolParser.reset();this.handlers.connecting();this.socket=new this.WebSocket(this._uri);this.socket.onopen=function(t){return e._onopen(t)};this.socket.onclose=function(t){return e._onclose(t)};this.socket.onmessage=function(t){return e._onmessage(t)};return this.socket.onerror=function(t){return e._onerror(t)}};e.prototype.disconnect=function(){this._connectionDesired=false;this._reconnectTimer.stop();if(!this._isSocketConnected()){return}this._disconnectionReason="manual";return this.socket.close()};e.prototype._scheduleReconnection=function(){if(!this._connectionDesired){return}if(!this._reconnectTimer.running){this._reconnectTimer.start(this._nextDelay);return this._nextDelay=Math.min(this.options.maxdelay,this._nextDelay*2)}};e.prototype.sendCommand=function(e){if(this.protocol==null){return}return this._sendCommand(e)};e.prototype._sendCommand=function(e){return this.socket.send(JSON.stringify(e))};e.prototype._closeOnError=function(){this._handshakeTimeout.stop();this._disconnectionReason="error";return this.socket.close()};e.prototype._onopen=function(e){var t;this.handlers.socketConnected();this._disconnectionReason="handshake-failed";t={command:"hello",protocols:[f,l]};t.ver=v;if(this.options.ext){t.ext=this.options.ext}if(this.options.extver){t.extver=this.options.extver}if(this.options.snipver){t.snipver=this.options.snipver}this._sendCommand(t);return this._handshakeTimeout.start(this.options.handshake_timeout)};e.prototype._onclose=function(e){this.protocol=0;this.handlers.disconnected(this._disconnectionReason,this._nextDelay);return this._scheduleReconnection()};e.prototype._onerror=function(e){};e.prototype._onmessage=function(e){return this.protocolParser.process(e.data)};return e}();var g;g={bind:function(e,t,n){if(e.addEventListener){return e.addEventListener(t,n,false)}else if(e.attachEvent){e[t]=1;return e.attachEvent("onpropertychange",function(e){if(e.propertyName===t){return n()}})}else{throw new Error("Attempt to attach custom event "+t+" to something which isn't a DOMElement")}},fire:function(e,t){var n;if(e.addEventListener){n=document.createEvent("HTMLEvents");n.initEvent(t,true,true);return document.dispatchEvent(n)}else if(e.attachEvent){if(e[t]){return e[t]++}}else{throw new Error("Attempt to fire custom event "+t+" on something which isn't a DOMElement")}}};n.bind=g.bind;n.fire=g.fire;var y;r=y=function(){function e(e,t){this.window=e;this.host=t}e.identifier="less";e.version="1.0";e.prototype.reload=function(e,t){if(this.window.less&&this.window.less.refresh){if(e.match(/\.less$/i)){return this.reloadLess(e)}if(t.originalPath.match(/\.less$/i)){return this.reloadLess(t.originalPath)}}return false};e.prototype.reloadLess=function(e){var t,n,r,i;n=function(){var e,n,r,i;r=document.getElementsByTagName("link");i=[];for(e=0,n=r.length;e<n;e++){t=r[e];if(t.href&&t.rel==="stylesheet/less"||t.rel.match(/stylesheet/)&&t.type.match(/^text\/(x-)?less$/)){i.push(t)}}return i}();if(n.length===0){return false}for(r=0,i=n.length;r<i;r++){t=n[r];t.href=this.host.generateCacheBustUrl(t.href)}this.host.console.log("LiveReload is asking LESS to recompile all stylesheets");this.window.less.refresh(true);return true};e.prototype.analyze=function(){return{disable:!!(this.window.less&&this.window.less.refresh)}};return e}();var b;i.Timer=b=function(){function e(e){var t=this;this.func=e;this.running=false;this.id=null;this._handler=function(){t.running=false;t.id=null;return t.func()}}e.prototype.start=function(e){if(this.running){clearTimeout(this.id)}this.id=setTimeout(this._handler,e);return this.running=true};e.prototype.stop=function(){if(this.running){clearTimeout(this.id);this.running=false;return this.id=null}};return e}();b.start=function(e,t){return setTimeout(t,e)};var w;s.Options=w=function(){function e(){this.host=null;this.port=35729;this.snipver=null;this.ext=null;this.extver=null;this.mindelay=1e3;this.maxdelay=6e4;this.handshake_timeout=5e3}e.prototype.set=function(e,t){switch(typeof this[e]){case"undefined":break;case"number":return this[e]=+t;default:return this[e]=t}};return e}();w.extract=function(e){var t,n,r,i,s,o,u,a,f,l,c,h,p;h=e.getElementsByTagName("script");for(a=0,l=h.length;a<l;a++){t=h[a];if((u=t.src)&&(r=u.match(/^[^:]+:\/\/(.*)\/z?livereload\.js(?:\?(.*))?$/))){s=new w;if(i=r[1].match(/^([^\/:]+)(?::(\d+))?$/)){s.host=i[1];if(i[2]){s.port=parseInt(i[2],10)}}if(r[2]){p=r[2].split("&");for(f=0,c=p.length;f<c;f++){o=p[f];if((n=o.split("=")).length>1){s.set(n[0].replace(/-/g,"_"),n.slice(1).join("="))}}}return s}}return null};var E,S,x,T,N,C,k;k=function(e){var t,n,r;if((n=e.indexOf("#"))>=0){t=e.slice(n);e=e.slice(0,n)}else{t=""}if((n=e.indexOf("?"))>=0){r=e.slice(n);e=e.slice(0,n)}else{r=""}return{url:e,params:r,hash:t}};T=function(e){var t;e=k(e).url;if(e.indexOf("file://")===0){t=e.replace(/^file:\/\/(localhost)?/,"")}else{t=e.replace(/^([^:]+:)?\/\/([^:\/]+)(:\d*)?\//,"/")}return decodeURIComponent(t)};C=function(e,t,n){var r,i,s,o,u;r={score:0};for(o=0,u=t.length;o<u;o++){i=t[o];s=x(e,n(i));if(s>r.score){r={object:i,score:s}}}if(r.score>0){return r}else{return null}};x=function(e,t){var n,r,i,s;e=e.replace(/^\/+/,"").toLowerCase();t=t.replace(/^\/+/,"").toLowerCase();if(e===t){return 1e4}n=e.split("/").reverse();r=t.split("/").reverse();s=Math.min(n.length,r.length);i=0;while(i<s&&n[i]===r[i]){++i}return i};N=function(e,t){return x(e,t)>0};E=[{selector:"background",styleNames:["backgroundImage"]},{selector:"border",styleNames:["borderImage","webkitBorderImage","MozBorderImage"]}];o.Reloader=S=function(){function e(e,t,n){this.window=e;this.console=t;this.Timer=n;this.document=this.window.document;this.importCacheWaitPeriod=200;this.plugins=[]}e.prototype.addPlugin=function(e){return this.plugins.push(e)};e.prototype.analyze=function(e){return results};e.prototype.reload=function(e,t){var n,r,i,s,o,u;this.options=t;if((o=(r=this.options).stylesheetReloadTimeout)==null){r.stylesheetReloadTimeout=15e3}u=this.plugins;for(i=0,s=u.length;i<s;i++){n=u[i];if(n.reload&&n.reload(e,t)){return}}if(t.liveCSS){if(e.match(/\.css$/i)){if(this.reloadStylesheet(e)){return}}}if(t.liveImg){if(e.match(/\.(jpe?g|png|gif)$/i)){this.reloadImages(e);return}}return this.reloadPage()};e.prototype.reloadPage=function(){return this.window.document.location.reload()};e.prototype.reloadImages=function(e){var t,n,r,i,s,o,u,a,f,l,c,h,p,d,v,m,g,y;t=this.generateUniqueString();d=this.document.images;for(o=0,l=d.length;o<l;o++){n=d[o];if(N(e,T(n.src))){n.src=this.generateCacheBustUrl(n.src,t)}}if(this.document.querySelectorAll){for(u=0,c=E.length;u<c;u++){v=E[u],r=v.selector,i=v.styleNames;m=this.document.querySelectorAll("[style*="+r+"]");for(a=0,h=m.length;a<h;a++){n=m[a];this.reloadStyleImages(n.style,i,e,t)}}}if(this.document.styleSheets){g=this.document.styleSheets;y=[];for(f=0,p=g.length;f<p;f++){s=g[f];y.push(this.reloadStylesheetImages(s,e,t))}return y}};e.prototype.reloadStylesheetImages=function(e,t,n){var r,i,s,o,u,a,f,l;try{s=e!=null?e.cssRules:void 0}catch(c){r=c}if(!s){return}for(u=0,f=s.length;u<f;u++){i=s[u];switch(i.type){case CSSRule.IMPORT_RULE:this.reloadStylesheetImages(i.styleSheet,t,n);break;case CSSRule.STYLE_RULE:for(a=0,l=E.length;a<l;a++){o=E[a].styleNames;this.reloadStyleImages(i.style,o,t,n)}break;case CSSRule.MEDIA_RULE:this.reloadStylesheetImages(i,t,n)}}};e.prototype.reloadStyleImages=function(e,t,n,r){var i,s,o,u,a,f=this;for(u=0,a=t.length;u<a;u++){s=t[u];o=e[s];if(typeof o==="string"){i=o.replace(/\burl\s*\(([^)]*)\)/,function(e,t){if(N(n,T(t))){return"url("+f.generateCacheBustUrl(t,r)+")"}else{return e}});if(i!==o){e[s]=i}}}};e.prototype.reloadStylesheet=function(e){var t,n,r,i,s,o,u,a,f,l,c,h,p,d,v,m=this;r=function(){var e,t,r,i;r=this.document.getElementsByTagName("link");i=[];for(e=0,t=r.length;e<t;e++){n=r[e];if(n.rel==="stylesheet"&&!n.__LiveReload_pendingRemoval){i.push(n)}}return i}.call(this);t=[];d=this.document.getElementsByTagName("style");for(o=0,l=d.length;o<l;o++){s=d[o];if(s.sheet){this.collectImportedStylesheets(s,s.sheet,t)}}for(u=0,c=r.length;u<c;u++){n=r[u];this.collectImportedStylesheets(n,n.sheet,t)}if(this.window.StyleFix&&this.document.querySelectorAll){v=this.document.querySelectorAll("style[data-href]");for(a=0,h=v.length;a<h;a++){s=v[a];r.push(s)}}this.console.log("LiveReload found "+r.length+" LINKed stylesheets, "+t.length+" @imported stylesheets");i=C(e,r.concat(t),function(e){return T(m.linkHref(e))});if(i){if(i.object.rule){this.console.log("LiveReload is reloading imported stylesheet: "+i.object.href);this.reattachImportedRule(i.object)}else{this.console.log("LiveReload is reloading stylesheet: "+this.linkHref(i.object));this.reattachStylesheetLink(i.object)}}else{this.console.log("LiveReload will reload all stylesheets because path '"+e+"' did not match any specific one");for(f=0,p=r.length;f<p;f++){n=r[f];this.reattachStylesheetLink(n)}}return true};e.prototype.collectImportedStylesheets=function(e,t,n){var r,i,s,o,u,a;try{o=t!=null?t.cssRules:void 0}catch(f){r=f}if(o&&o.length){for(i=u=0,a=o.length;u<a;i=++u){s=o[i];switch(s.type){case CSSRule.CHARSET_RULE:continue;case CSSRule.IMPORT_RULE:n.push({link:e,rule:s,index:i,href:s.href});this.collectImportedStylesheets(e,s.styleSheet,n);break;default:break}}}};e.prototype.waitUntilCssLoads=function(e,t){var n,r,i,s=this;n=false;r=function(){if(n){return}n=true;return t()};e.onload=function(){s.console.log("LiveReload: the new stylesheet has finished loading");s.knownToSupportCssOnLoad=true;return r()};if(!this.knownToSupportCssOnLoad){(i=function(){if(e.sheet){s.console.log("LiveReload is polling until the new CSS finishes loading...");return r()}else{return s.Timer.start(50,i)}})()}return this.Timer.start(this.options.stylesheetReloadTimeout,r)};e.prototype.linkHref=function(e){return e.href||e.getAttribute("data-href")};e.prototype.reattachStylesheetLink=function(e){var t,n,r=this;if(e.__LiveReload_pendingRemoval){return}e.__LiveReload_pendingRemoval=true;if(e.tagName==="STYLE"){t=this.document.createElement("link");t.rel="stylesheet";t.media=e.media;t.disabled=e.disabled}else{t=e.cloneNode(false)}t.href=this.generateCacheBustUrl(this.linkHref(e));n=e.parentNode;if(n.lastChild===e){n.appendChild(t)}else{n.insertBefore(t,e.nextSibling)}return this.waitUntilCssLoads(t,function(){var n;if(/AppleWebKit/.test(navigator.userAgent)){n=5}else{n=200}return r.Timer.start(n,function(){var n;if(!e.parentNode){return}e.parentNode.removeChild(e);t.onreadystatechange=null;return(n=r.window.StyleFix)!=null?n.link(t):void 0})})};e.prototype.reattachImportedRule=function(e){var t,n,r,i,s,o,u,a,f=this;u=e.rule,n=e.index,r=e.link;o=u.parentStyleSheet;t=this.generateCacheBustUrl(u.href);i=u.media.length?[].join.call(u.media,", "):"";s='@import url("'+t+'") '+i+";";u.__LiveReload_newHref=t;a=this.document.createElement("link");a.rel="stylesheet";a.href=t;a.__LiveReload_pendingRemoval=true;if(r.parentNode){r.parentNode.insertBefore(a,r)}return this.Timer.start(this.importCacheWaitPeriod,function(){if(a.parentNode){a.parentNode.removeChild(a)}if(u.__LiveReload_newHref!==t){return}o.insertRule(s,n);o.deleteRule(n+1);u=o.cssRules[n];u.__LiveReload_newHref=t;return f.Timer.start(f.importCacheWaitPeriod,function(){if(u.__LiveReload_newHref!==t){return}o.insertRule(s,n);return o.deleteRule(n+1)})})};e.prototype.generateUniqueString=function(){return"livereload="+Date.now()};e.prototype.generateCacheBustUrl=function(e,t){var n,r,i,s,o;if(t==null){t=this.generateUniqueString()}o=k(e),e=o.url,n=o.hash,r=o.params;if(this.options.overrideURL){if(e.indexOf(this.options.serverURL)<0){i=e;e=this.options.serverURL+this.options.overrideURL+"?url="+encodeURIComponent(e);this.console.log("LiveReload is overriding source URL "+i+" with "+e)}}s=r.replace(/(\?|&)livereload=(\d+)/,function(e,n){return""+n+t});if(s===r){if(r.length===0){s="?"+t}else{s=""+r+"&"+t}}return e+s+n};return e}();var d,L,w,S,b;d=t.Connector;b=i.Timer;w=s.Options;S=o.Reloader;u.LiveReload=L=function(){function e(e){var t=this;this.window=e;this.listeners={};this.plugins=[];this.pluginIdentifiers={};this.console=this.window.location.href.match(/LR-verbose/)&&this.window.console&&this.window.console.log&&this.window.console.error?this.window.console:{log:function(){},error:function(){}};if(!(this.WebSocket=this.window.WebSocket||this.window.MozWebSocket)){console.error("LiveReload disabled because the browser does not seem to support web sockets");return}if(!(this.options=w.extract(this.window.document))){console.error("LiveReload disabled because it could not find its own <SCRIPT> tag");return}this.reloader=new S(this.window,this.console,b);this.connector=new d(this.options,this.WebSocket,b,{connecting:function(){},socketConnected:function(){},connected:function(e){var n;if(typeof (n=t.listeners).connect==="function"){n.connect()}t.log("LiveReload is connected to "+t.options.host+":"+t.options.port+" (protocol v"+e+").");return t.analyze()},error:function(e){if(e instanceof h){return console.log(""+e.message+".")}else{return console.log("LiveReload internal error: "+e.message)}},disconnected:function(e,n){var r;if(typeof (r=t.listeners).disconnect==="function"){r.disconnect()}switch(e){case"cannot-connect":return t.log("LiveReload cannot connect to "+t.options.host+":"+t.options.port+", will retry in "+n+" sec.");case"broken":return t.log("LiveReload disconnected from "+t.options.host+":"+t.options.port+", reconnecting in "+n+" sec.");case"handshake-timeout":return t.log("LiveReload cannot connect to "+t.options.host+":"+t.options.port+" (handshake timeout), will retry in "+n+" sec.");case"handshake-failed":return t.log("LiveReload cannot connect to "+t.options.host+":"+t.options.port+" (handshake failed), will retry in "+n+" sec.");case"manual":break;case"error":break;default:return t.log("LiveReload disconnected from "+t.options.host+":"+t.options.port+" ("+e+"), reconnecting in "+n+" sec.")}},message:function(e){switch(e.command){case"reload":return t.performReload(e);case"alert":return t.performAlert(e)}}})}e.prototype.on=function(e,t){return this.listeners[e]=t};e.prototype.log=function(e){return this.console.log(""+e)};e.prototype.performReload=function(e){var t,n;this.log("LiveReload received reload request: "+JSON.stringify(e,null,2));return this.reloader.reload(e.path,{liveCSS:(t=e.liveCSS)!=null?t:true,liveImg:(n=e.liveImg)!=null?n:true,originalPath:e.originalPath||"",overrideURL:e.overrideURL||"",serverURL:"http://"+this.options.host+":"+this.options.port})};e.prototype.performAlert=function(e){return alert(e.message)};e.prototype.shutDown=function(){var e;this.connector.disconnect();this.log("LiveReload disconnected.");return typeof (e=this.listeners).shutdown==="function"?e.shutdown():void 0};e.prototype.hasPlugin=function(e){return!!this.pluginIdentifiers[e]};e.prototype.addPlugin=function(e){var t,n=this;if(this.hasPlugin(e.identifier)){return}this.pluginIdentifiers[e.identifier]=true;t=new e(this.window,{_livereload:this,_reloader:this.reloader,_connector:this.connector,console:this.console,Timer:b,generateCacheBustUrl:function(e){return n.reloader.generateCacheBustUrl(e)}});this.plugins.push(t);this.reloader.addPlugin(t)};e.prototype.analyze=function(){var e,t,n,r,i,s;if(!(this.connector.protocol>=7)){return}n={};s=this.plugins;for(r=0,i=s.length;r<i;r++){e=s[r];n[e.constructor.identifier]=t=(typeof e.analyze==="function"?e.analyze():void 0)||{};t.version=e.constructor.version}this.connector.sendCommand({command:"info",plugins:n,url:this.window.location.href})};return e}();var g,L,A;g=n;L=window.LiveReload=new u.LiveReload(window);for(A in window){if(A.match(/^LiveReloadPlugin/)){L.addPlugin(window[A])}}L.addPlugin(r);L.on("shutdown",function(){return delete window.LiveReload});L.on("connect",function(){return g.fire(document,"LiveReloadConnect")});L.on("disconnect",function(){return g.fire(document,"LiveReloadDisconnect")});g.bind(document,"LiveReloadShutDown",function(){return L.shutDown()})})()`),
}
