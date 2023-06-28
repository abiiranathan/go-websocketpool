package websocketpool

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	ErrPoolClosed       = errors.New("websocket pool is closed")
	ErrConnectionClosed = errors.New("websocket connection closed")
)

// Implements a pool of buffered channels to send and receive
// websocket messages.
type WebSocketPool struct {
	dialer       *websocket.Dialer
	connection   *websocket.Conn
	connectionMu sync.RWMutex
	poolSize     int
	queue        chan []byte
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// Create a new websocket pool and connect to remote server.
func NewWebSocketPool(url string, poolSize int) (*WebSocketPool, error) {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	wp := &WebSocketPool{
		dialer:     dialer,
		connection: conn,
		poolSize:   poolSize,
		queue:      make(chan []byte, poolSize),
		stopCh:     make(chan struct{}),
	}

	wp.startWorker()
	return wp, nil
}

// Helper function to send messages.
func (wp *WebSocketPool) startWorker() {
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		for {
			select {
			case msg := <-wp.queue:
				wp.send(msg)
			case <-wp.stopCh:
				return
			}
		}
	}()
}

func (wp *WebSocketPool) send(message []byte) error {
	wp.connectionMu.RLock()

	if wp.connection == nil {
		wp.connectionMu.RUnlock()
		return ErrConnectionClosed
	}

	conn := wp.connection
	wp.connectionMu.RUnlock()

	conn.SetWriteDeadline(time.Now().Add(time.Second))
	err := conn.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		wp.connectionMu.Lock()
		if conn == wp.connection {
			wp.connection = nil
		}
		wp.connectionMu.Unlock()
	}
	return err
}

func (wp *WebSocketPool) Send(message []byte) error {
	select {
	case wp.queue <- message:
		return nil
	default:
		return ErrPoolClosed
	}
}

func (wp *WebSocketPool) ReadMessage() (int, []byte, error) {
	wp.connectionMu.RLock()
	defer wp.connectionMu.RUnlock()

	if wp.connection == nil {
		return 0, nil, ErrConnectionClosed
	}

	return wp.connection.ReadMessage()
}

func (wp *WebSocketPool) Reconnect() error {
	wp.connectionMu.Lock()
	defer wp.connectionMu.Unlock()

	if wp.connection != nil {
		wp.connection.Close()
		wp.connection = nil
	}

	conn, _, err := wp.dialer.Dial(wp.connection.RemoteAddr().String(), nil)
	if err != nil {
		return err
	}

	wp.connection = conn
	return nil
}

func (wp *WebSocketPool) Close() {
	close(wp.stopCh)
	wp.wg.Wait()

	wp.connectionMu.Lock()
	defer wp.connectionMu.Unlock()

	if wp.connection != nil {
		wp.connection.Close()
		wp.connection = nil
	}
}
