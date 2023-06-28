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

var readTimeout = time.Second * 10
var writeTimeout = time.Second * 10
var reconnectDelay = time.Second

// Implements a pool of buffered channels to send and receive
// websocket messages.
type WebSocketPool struct {
	serverAddr   string
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
		serverAddr: url,
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

	conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	err := conn.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		// Try reconnecting
		time.Sleep(reconnectDelay)
		if err := wp.Reconnect(); err != nil {
			return err
		}

		// Retry sending after successful reconnection
		conn = wp.connection
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		err = conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return err
		}
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

	wp.connection.SetReadDeadline(time.Now().Add(readTimeout))
	return wp.connection.ReadMessage()
}

func (wp *WebSocketPool) Reconnect() error {
	wp.connectionMu.Lock()
	defer wp.connectionMu.Unlock()

	if wp.connection != nil {
		wp.connection.Close()
		wp.connection = nil
	}

	conn, _, err := wp.dialer.Dial(wp.serverAddr, nil)
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

// Set the time taken to read from a connection.
func SetReadTimeout(timeout time.Duration) {
	readTimeout = timeout
}

// Set the time taken to write to a connection.
func SetWriteTimeout(timeout time.Duration) {
	writeTimeout = timeout
}

func SetRetryDelay(delay time.Duration) {
	reconnectDelay = delay
}
