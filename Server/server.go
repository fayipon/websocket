package main

import (
	"fmt"
	"encoding/json"
	"net/http"
    "io/ioutil"
	"time"
	"strings"
	"strconv"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
)

const (
	ChatApiUrl	 string = "https://shopping166.net/chat/"
)


// 是否使用DEBUG
var isDebug = true

//建立客戶端管理者
var manager = ClientManager{
	broadcast:  make(chan []byte),
	privatemessage:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	clients:    make(map[*Client]bool),
}

//客戶端管理
type ClientManager struct {
	//客戶端 map 儲存並管理所有的長連線client，線上的為true，不在的為false
	clients map[*Client]bool

	//web端傳送來的的message我們用broadcast來接收，並最後分發給所有的client
	broadcast chan []byte

	//web端傳送來的的message我們用broadcast來接收，並最後分發給所有的client
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
	buyer_id string
	//使用者email
	email string
	//連線的socket
	socket *websocket.Conn
	//傳送的訊息
	send chan []byte
}

//會把Message格式化成json
type Message struct {
	//訊息struct
	Sender    string `json:"sender,omitempty"`    //傳送者 email
	Revicer string `json:revicer,omitempty`

	// 比照Laravel 格式
	Data   string `json:"data,omitempty"`   //內容
	Message   string `json:"message,omitempty"`   //訊息
	Status   string `json:"status,omitempty"`   //狀態
}

type ReviceMessage struct {
	Action  string `json:"action"`
	Email 	string `json:"email"`
	Token 	string `json:"token"`
	Info 	string `json:"info"`
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

			// debug , 發送內容
			DebugLog(string(message))

			//遍歷已經連線的客戶端，把訊息傳送給他們
			for conn := range manager.clients {
				select {
					case conn.send <- message:
					default:
						close(conn.send)
						delete(manager.clients, conn)
				}
			}

		// 私聊
		case message := <-manager.privatemessage:
			
			DebugLog("privatemessage")
			DebugLog("================")

			for conn := range manager.clients {
				
				var revice_message ReviceMessage
				json.Unmarshal(message, &revice_message)
				
				if (string(conn.buyer_id) == revice_message.Revicer) {

					select {
					case conn.send <- message:
					default:
					}
					
					DebugLog("Revicer : " + revice_message.Revicer)
					DebugLog("Private message DONE")
				}

				if (string(conn.email) == revice_message.Email) {

					select {
					case conn.send <- message:
					default:
					}
					
					DebugLog("Sender : " + revice_message.Email)
					DebugLog("Private message DONE")
				}

			}

			DebugLog("================")

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

		// 依據Action , 做區別處理
		switch revice_message.Action {
			// 注冊
			case "register":
				if c.email == "" {
	
					// 如果還未注冊
					DebugLog("Register Done")
				} else {
					DebugLog("Register Already")
				}
	
				jsonMessage, _ := json.Marshal(&InitMessage{
						Action: "init",
						Email:revice_message.Email, 
						Token:revice_message.Token, 
						Revicer: 111, 
						Content: string(data)})
						
				manager.privatemessage <- jsonMessage
	
		// 心跳
		case "heartbeat":

			timeUnix := time.Now().Format("2006-01-02 15:04:05")

			jsonMessage, _ := json.Marshal(&InitMessage{
				Action: "heartbeat",
				Email:revice_message.Email, 
				Token:revice_message.Token, 
				Sender: buyerOnline.BuyerId, 
				Revicer: buyerOnline.BuyerId, 
				Content: timeUnix})
				
			manager.privatemessage <- jsonMessage
	
		// 私聊
		case "private":

			timeUnix := time.Now().Format("2006-01-02 15:04:05")

			jsonMessage, _ := json.Marshal(&ChatMessage{
					Action:  "private",
					Email:revice_message.Email, 
					Token:revice_message.Token, 
					Revicer: revice_message.Revicer, 
					Content:revice_message.Info,
					Datetime: timeUnix})

			manager.privatemessage <- jsonMessage
			

		// 廣播 , 暫時沒有場景
		case "broadcast":
				
			timeUnix := time.Now().Format("2006-01-02 15:04:05")

			jsonMessage, _ := json.Marshal(&InitMessage{
				Action: "broadcast",
				Email:revice_message.Email, 
				Token:revice_message.Token,
				Content: revice_message.Info,
				Datetime: timeUnix})

			manager.broadcast <- jsonMessage

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
