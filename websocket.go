package main

import (
	"code.google.com/p/go.net/websocket"
    "github.com/nu7hatch/gouuid"
	"net/http"
	"fmt"
    "time"
)

type connection struct {
    // The websocket connection.
    ws *websocket.Conn

    // Buffered channel of outbound messages.
    send chan map[string]string

    //Random UUID for this channel.
    id *uuid.UUID
}

type hub struct {
    // Registered connections.
    connections map[*connection]bool

    // Inbound messages from the connections.
    broadcast chan map[string]string

    // Register requests from the connections.
    register chan *connection

    // Unregister requests from connections.
    unregister chan *connection
}

func (h *hub) run() {
    for {
        select {
        case c := <-h.register:
            h.connections[c] = true
            fmt.Println(c.id,"New connection");
        case c := <-h.unregister:
            delete(h.connections, c)
            close(c.send)
            go c.ws.Close()
            fmt.Println(c.id,"Unregister connection");
        case m := <-h.broadcast:
            for c := range h.connections {
                select {
                case c.send <- m:
                default:
                    delete(h.connections, c)
                    close(c.send)
                    go c.ws.Close()
                    fmt.Println(c.id,"Closed connection");
                }
            }
        }
    }
}

func clientConnect(ws *websocket.Conn, h hub){

        //Generate random UUID for connection
        id, _ := uuid.NewV4();

		c := &connection{
                send: make(chan map[string]string),
                ws: ws,
                id: id,
        }

		h.register <- c
        go clientWriter(c, h);
		clientReader(c, h);
        h.unregister <- c
}

func clientWriter(c *connection, h hub){

    msg := make(map[string]string)
    msg["event"] = "hello"
    msg["data"] = "Hello World"

    websocket.JSON.Send(c.ws, msg)
    fmt.Println(c.id, "Sent:", msg);

    for m := range c.send {
        fmt.Println(c.id, "Sent:", m);
        err := websocket.JSON.Send(c.ws, m)

        if err != nil {
            fmt.Println(c.id,"Client send error", err);
            return
        }
    }
}

func clientReader(c *connection, h hub){

    for {
        var message string
        err := websocket.Message.Receive(c.ws, &message)

        if err != nil {
            fmt.Println(c.id,"Client reader error", err);
            return
        }

        msg := make(map[string]string)
        msg["event"] = "msg"
        msg["data"] = message
        fmt.Println(c.id,"Recv:", msg);

        h.broadcast <- msg
    }
}

func pingBroadcast(h hub){
    for {
        msg := make(map[string]string)
        msg["event"] = "ping"
        h.broadcast <- msg
        time.Sleep(5 * time.Second)
    }
}

func main() {

	//Define channels
	var h = hub{
	    broadcast:   make(chan map[string]string),
	    register:    make(chan *connection),
	    unregister:  make(chan *connection),
	    connections: make(map[*connection]bool),
	}

	//Start up message hub
	go h.run()

    //Start up ping broadcaster
    go pingBroadcast(h)

	//Define anonymous function to allow passing of connectEvents channel
	//into web socket handler.
	handler := func(ws *websocket.Conn) {
		clientConnect(ws, h);
	}

	http.Handle("/echo", websocket.Handler(handler))
	http.Handle("/", http.FileServer(http.Dir(".")))

	fmt.Println("Started...");
	err := http.ListenAndServe("[::]:8080", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
