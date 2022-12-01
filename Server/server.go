package main

import (
	"fmt"
	"encoding/json"
	"net/http"
  //  "io/ioutil"
	"time"
  //  "strings"
  //	"strconv"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
)

const (
	ChatApiUrl	 string = "https://shopping168.net/chatapi/"
)


// 是否使用DEBUG
var isDebug = true

//客戶端管理
type ClientManager struct {
	//客戶端 map 儲存並管理所有的長連線client，線上的為true，不在的為false
	clients map[*Client]bool

	//broadcast
	broadcast chan []byte

	//channel
	channel chan []byte

	//privatemessage
	privatemessage chan []byte

	//新建立的長連線client
	register chan *Client

	//新登出的長連線client
	unregister chan *Client
}

//客戶端 Client
type Client struct {
	//使用者id
	id string
	//使用者id
	account_id string
	//account
	account string
	// 當前頻道
	channel string
	//連線的socket
	socket *websocket.Conn
	//傳送的訊息
	send chan []byte
}

//建立客戶端管理者
var manager = ClientManager{
	broadcast:  make(chan []byte),
	channel:  make(chan []byte),
	privatemessage:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	clients:    make(map[*Client]bool),
}

//會把Message格式化成json
type Message struct {
	//訊息struct
	Sender    string `json:"sender,omitempty"`    //傳送者 email
	Revicer string `json:revicer,omitempty`

	// 比照ThinkPHP 格式
	Data   string `json:"data,omitempty"`   //內容
	Message   string `json:"message,omitempty"`   //訊息
	Status   string `json:"status,omitempty"`   //狀態
}

// 接收
type ReviceMessage struct {
	Action  string `json:"action"`
	Data 	string `json:"data,omitempty"`
	Channel string `json:"channel,omitempty"`
	Account string `json:"account,omitempty"`
	Id string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
	Status 	string `json:"status,omitempty"`
}

// 回覆 廣播
type ReplyBroadcastMessage struct {
	Action  string `json:"action"` 
	Data 	string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Status 	string `json:"status,omitempty"`
}

// 回覆 頻道
type ReplyChannelMessage struct {
	Action  string `json:"action"` 
	Channel string `json:"channel,omitempty"`
	Data 	string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Status 	string `json:"status,omitempty"`
}

// 回覆 私訊
type ReplyPrivateMessage struct {
	Action  string `json:"action"` 
	Revicer string `json:"revicer,omitempty"`
	Data 	string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Status 	string `json:"status,omitempty"`
}


func (manager *ClientManager) start() {
	for {
		select {
		//如果有新的連線接入,就通過channel把連線傳遞給conn
		case conn := <-manager.register:
			//把客戶端的連線設定為true
			manager.clients[conn] = true
			//把返回連線成功的訊息json格式化

			// debug , 發送內容
			DebugLog("Connect Success")

		//如果連線斷開了
		case conn := <-manager.unregister:
			//判斷連線的狀態，如果是true,就關閉send，刪除連線client的值
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				
				
				// debug , 發送內容
				DebugLog("Disconnect")
			}

		//廣播
		case message := <-manager.broadcast:

			//遍歷已經連線的客戶端，把訊息傳送給他們
			for conn := range manager.clients {
					
					select {
						case conn.send <- message:
						default:
							close(conn.send)
							delete(manager.clients, conn)
					}
			}

		//頻道
		case message := <-manager.channel:

			var revice_message ReviceMessage
			json.Unmarshal(message, &revice_message)

			//遍歷已經連線的客戶端，把訊息傳送給他們
			for conn := range manager.clients {
				// 判斷頻道是否一致
				if (conn.channel == revice_message.Channel) {
					select {
						case conn.send <- message:
						default:
							close(conn.send)
							delete(manager.clients, conn)
					}
				}

			}

		// 私聊
		case message := <-manager.privatemessage:
			
			var revice_message ReplyPrivateMessage
			json.Unmarshal(message, &revice_message)

			for conn := range manager.clients {

				// 判斷私聊是否一致
				if (conn.account_id == revice_message.Revicer) {
					select {
						case conn.send <- message:
						default:
							close(conn.send)
							delete(manager.clients, conn)
					}
				}
			}

		}
	}
}

//定義客戶端管理的send方法
func (manager *ClientManager) send(message []byte, ignore *Client) {
	for conn := range manager.clients {
		//不給遮蔽的連線傳送訊息
		if conn != ignore {
			conn.send <- message
		}
	}
}

//定義客戶端結構體的read方法
func (c *Client) read() {
	defer func() {
		manager.unregister <- c
		c.socket.Close()
	}()

	for {
		//讀取訊息
		_, message, err := c.socket.ReadMessage()
		//如果有錯誤資訊，就登出這個連線然後關閉
		if err != nil {
			manager.unregister <- c
			c.socket.Close()
			break
		}

		DebugLog("ReviceMessage")
		
		DebugLog("=================")
		// debug , 內容
		DebugLog(string(message))

		var revice_message ReviceMessage
		
		json.Unmarshal(message, &revice_message)

		fmt.Println(revice_message)
		
		////////////////////////////////////////


		// 依據Action , 做區別處理
		switch revice_message.Action {
	
		// 心跳
		case "heartbeat":

			DebugLog("heartbeat")
			timeUnix := time.Now().Format("2006-01-02 15:04:05")

			jsonMessage, _ := json.Marshal(&ReplyPrivateMessage{
				Action: "heartbeat",
				Data: timeUnix,
				Status: "1"})
				
			manager.privatemessage <- jsonMessage
	
		// 頻道
		case "register":

			c.channel	 = revice_message.Channel
			c.account	 = revice_message.Account
			c.account_id = revice_message.Id

			jsonMessage, _ := json.Marshal(&ReplyPrivateMessage{
				Action: "register",
				Revicer: c.account_id,
				Message: "regist success",
				Status:  "1"})

			manager.privatemessage <- jsonMessage

		// 私聊
		case "private":
			DebugLog("private")
				
		// 廣播
		case "broadcast":
				
			DebugLog("broadcast")
			jsonMessage, _ := json.Marshal(&ReplyBroadcastMessage{
				Action: "broadcast",
				Data: revice_message.Data,
				Message: string(time.Now().Format("2006-01-02 15:04:05")),
				Status:  "1"})

			manager.broadcast <- jsonMessage

		// 頻道
		case "channel":
					
			DebugLog("channel")

			jsonMessage, _ := json.Marshal(&ReplyChannelMessage{
				Action: "channel",
				Channel: revice_message.Channel,
				Data: revice_message.Data,
				Message: string(time.Now().Format("2006-01-02 15:04:05")),
				Status:  "1"})

			manager.channel <- jsonMessage

		default:
			DebugLog("default")
		}

		DebugLog("================")
	}
}

func (c *Client) write() {
	defer func() {
		c.socket.Close()
	}()

	for {
		select {
		//從send裡讀訊息
		case message, ok := <-c.send:
			//如果沒有訊息
			if !ok {
				c.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			//有訊息就寫入，傳送給web端
			c.socket.WriteMessage(websocket.TextMessage, message)
			
			DebugLog("SendMessage")
		}
	}
}

func main() {
	
	SystemLog("Starting application...")
	
	if (isDebug) {
		SystemLog("DEBUG MODE ON")
	} else {
		SystemLog("DEBUG MODE OFF")
	}

	//開一個goroutine執行開始程式
	go manager.start()
	//註冊預設路由為 /ws ，並使用wsHandler這個方法
	http.HandleFunc("/ws", wsHandler)
	//監聽本地的8011埠
	http.ListenAndServe(":8011", nil)
}

func wsHandler(res http.ResponseWriter, req *http.Request) {
	//將http協議升級成websocket協議
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(res, req, nil)
	if err != nil {
		http.NotFound(res, req)
		return
	}

	//每一次連線都會新開一個client，client.id通過uuid生成保證每次都是不同的
	client := &Client{id: uuid.Must(uuid.NewV4()).String(), socket: conn, send: make(chan []byte)}
	//註冊一個新的連結
	manager.register <- client

	//啟動協程收web端傳過來的訊息
	go client.read()
	//啟動協程把訊息返回給web端
	go client.write()
}

////////////////////////////////
//
//   custom function
//
////////////////////////////////

// debug log 
func DebugLog(message string) {
	if (isDebug) {
		fmt.Println("[" + string(time.Now().Format("2006-01-02 15:04:05")) + "] " + message)
	}
}

// System log 
func SystemLog(message string) {
	fmt.Println("[" + string(time.Now().Format("2006-01-02 15:04:05")) + "] " + message)
}
