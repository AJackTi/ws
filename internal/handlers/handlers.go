package handlers

import (
	"fmt"
	"github.com/CloudyKit/jet/v6"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sort"
)

var wsChan = make(chan WsPayload)

var clients = make(map[WebSocketConnection]string)

// views is the jet view set
var views = jet.NewSet(
	jet.NewOSFileSystemLoader("./html"),
	jet.InDevelopmentMode(),
)

// upgradeConnection is the websocket upgrader from gorilla/websockets
var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketConnection struct {
	*websocket.Conn
}

// WsJsonResponse defines the response sent back from websocket
type WsJsonResponse struct {
	Action         string   `json:"action"`
	Message        string   `json:"message"`
	MessageType    string   `json:"message_type"`
	ConnectedUsers []string `json:"connected_users"`
}

type WsPayload struct {
	Action   string              `json:"action"`
	Username string              `json:"username"`
	Message  string              `json:"message"`
	Conn     WebSocketConnection `json:"-"`
}

// WsEndpoint upgrades connection to websocket
func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	log.Println("Client connected to endpoint")

	var response WsJsonResponse
	response.Message = `<em><small>Connected to server</small></em>`

	conn := WebSocketConnection{ws}
	clients[conn] = ""

	err = ws.WriteJSON(response)
	if err != nil {
		log.Println(err)
	}

	go ListenForWs(&conn)
}

func ListenForWs(conn *WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()

	var payload WsPayload

	for {
		err := conn.ReadJSON(&payload)
		if err != nil {
			// do nothing
		} else {
			payload.Conn = *conn
			wsChan <- payload
		}
	}
}

func ListenToWsChannel() {
	var response WsJsonResponse

	for {
		e := <-wsChan

		switch e.Action {
		case "username":
			// get a list of all users and send it back via broadcast
			clients[e.Conn] = e.Username
			users := getUserList()
			response.Action = "list_users"
			response.ConnectedUsers = users
			broadcastToAll(response)

		case "left":
			response.Action = "list_users"
			delete(clients, e.Conn)
			users := getUserList()
			response.ConnectedUsers = users
			broadcastToAll(response)

		case "broadcast":
			response.Action = "broadcast"
			response.Message = fmt.Sprintf("<strong>%s</strong>: %s", e.Username, e.Message)
			broadcastToAll(response)

		}

		//response.Action = "Got here"
		//response.Message = fmt.Sprintf("Some message, and action was %s", e.Action)
		//broadcastToAll(response)
	}
}

func getUserList() []string {
	var userList []string
	for _, x := range clients {
		if x != "" {
			userList = append(userList, x)
		}
	}
	sort.Strings(userList)
	return userList
}

func broadcastToAll(response WsJsonResponse) {
	for client := range clients {
		err := client.WriteJSON(response)
		if err != nil {
			log.Println("websocket err")
			_ = client.Close()
			delete(clients, client)
		}
	}
}

// Home renders the home page
func Home(w http.ResponseWriter, r *http.Request) {
	err := renderPage(w, "home.jet", nil)
	if err != nil {
		log.Println(err)
	}
}

func renderPage(w http.ResponseWriter, tmpl string, data jet.VarMap) error {
	view, err := views.GetTemplate(tmpl)
	if err != nil {
		log.Println(err)
		return err
	}

	err = view.Execute(w, data, nil)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}
