// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package GoPlayer

import (
	"github.com/gorilla/websocket"
	"net/http"
	"time"
	"strings"

)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool { return true },
}

// connection is an middleman between the websocket connection and the hub.
type connection struct {
	// The websocket connection.
	ws *websocket.Conn
	// Buffered channel of outbound messages.
	send chan []byte
	metadata chan []byte
	error_channel chan *Error
}


// write writes a message with the given message type and payload.
func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (c *connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer c.Close()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.BinaryMessage, message); err != nil {
				return
			}
		case <-ticker.C:

			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		case metadata, ok:= <- c.metadata:
		if(ok) {
			c.write(websocket.TextMessage, metadata)
		}

		case error, ok:= <- c.error_channel:
		if(ok){
			error_object,err:= error.JSON();
			if(err==nil){
			c.write(websocket.TextMessage,error_object)
			}
			if(error.Level==1){
				return
			}
		}

		}
	}
}

// serverWs handles websocket requests from the peer.
func serveWs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	player, err:=GetPlayerInstance();
	if(err != nil){
		log.Error("No player found")
		return
	}
	client_id:=r.FormValue("client_id");
	access_token:=r.FormValue("access_token")
	model_id:=r.FormValue("model_id")
	player.log.Debug("serveWs")
	player.log.Debug("client_id: ",client_id)
	player.log.Debug("access_token: ",access_token)
	player.log.Debug("model_id: ",model_id)
	arr :=strings.Split(r.URL.String(),"/")
	player.log.Info("arr: ",arr)
	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		player.log.Error("ws upgrate error: ",err)
		return
	}
	player.log.Debug("ws upgrated")
	c := &connection{
		send: make(chan []byte, 256),
		ws: ws,metadata:make(chan []byte),
		error_channel: make(chan *Error),
	}
	player.log.Debug("connection created")


	if(player.streams_map[arr[len(arr)-1]] != nil) {
		player.streams_map[arr[len(arr)-1]].register <- c
	}else{
		player.log.Error("no hub found!!!")
		return
	}
	player.log.Debug("connection complete")
	go c.writePump()
}

func (c *connection)Close(){
	c.write(websocket.CloseMessage, []byte{})
		c.ws.Close()

	if(c.send != nil){
		close(c.send)
		c.send=nil
	}
	if(c.metadata != nil){
		close(c.metadata)
		c.metadata=nil
	}
	if(c.error_channel != nil){
		close(c.error_channel)
		c.error_channel=nil
	}
}
