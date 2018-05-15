package p2p

import (
	"bufio"
	"fmt"
	"net"
	"sync"

	"encoding/hex"
	"log"

	"time"

	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
)

type loggerWriter struct {
}

func (l loggerWriter) Write(p []byte) (n int, err error) {

	length := len(p)

	fmt.Printf("\t\t[%d] data [%s]\n", length, hex.EncodeToString(p))

	return length, nil
}

func NewClient(p2pAddr string, chainID eos.SHA256Bytes, networkVersion int16) *Client {
	c := &Client{
		p2pAddress:     p2pAddr,
		ChainID:        chainID,
		NetworkVersion: networkVersion,
	}
	c.NodeID = chainID
	return c
}

type Client struct {
	handlers     []Handler
	handlersLock sync.Mutex
	p2pAddress   string
	//API            *eos.API
	ChainID        eos.SHA256Bytes
	NetworkVersion int16
	Conn           net.Conn
	NodeID         eos.SHA256Bytes
}

func (c *Client) Connect() (err error) {

	//hi, err := c.getHandshakeInfo()
	//if err != nil {
	//	return nil, err
	//}
	//
	//handshakeInfo = &hi

	conn, err := net.Dial("tcp", c.p2pAddress)
	if err != nil {
		return err
	}

	c.Conn = conn

	if err := c.setupFlow(); err != nil {
		return err
	}

	fmt.Println("Connected to: ", c.p2pAddress)
	ready := make(chan bool)
	go c.handleConnection(&Route{From: c.p2pAddress}, ready)
	<-ready

	if err := c.SendHandshake(&HandshakeInfo{
		HeadBlockNum:             0,
		LastIrreversibleBlockNum: 0,
	}); err != nil {
		return err
	}

	return nil
}

func (c *Client) RegisterHandler(h Handler) {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()

	c.handlers = append(c.handlers, h)
}

func (c *Client) UnregisterHandler(h Handler) {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()

	var newHandlers []Handler
	for _, handler := range c.handlers {
		if handler != h {
			newHandlers = append(newHandlers, handler)
		}
	}
	c.handlers = newHandlers
}

func (c *Client) setupFlow() error {

	initHandler := HandlerFunc(func(processable PostProcessable) {
		msg, ok := processable.P2PMessageEnvelope.P2PMessage.(*eos.HandshakeMessage)
		if !ok {
			return
		}

		//if blockHeight == msg.HeadNum {
		hInfo := HandshakeInfo{
			HeadBlockNum:             msg.HeadNum,
			HeadBlockID:              msg.HeadID,
			HeadBlockTime:            msg.Time.Time,
			LastIrreversibleBlockNum: msg.LastIrreversibleBlockNum,
			LastIrreversibleBlockID:  msg.LastIrreversibleBlockID,
		}
		if err := c.SendHandshake(&hInfo); err != nil {
			log.Println("Failed sending handshake:", err)
		}

		//} else {
		//	err := c.SendSyncRequest(blockHeight, msg.HeadNum)
		//	if err != nil {
		//		log.Fatal(err)
		//	}
		//}

	})
	c.RegisterHandler(initHandler)

	return nil
}

//func (c *Client) getHandshakeInfo() (info HandshakeInfo, err error) {
//
//	peerInfo, err := c.API.GetInfo()
//	if err != nil {
//		return
//	}
//
//	fmt.Println("Peer info: ", peerInfo)
//
//	blockInfo, err := c.API.GetBlockByNum(uint64(peerInfo.LastIrreversibleBlockNum))
//	if err != nil {
//		return
//	}
//
//	info = HandshakeInfo{
//		HeadBlockNum:             peerInfo.HeadBlockNum,
//		HeadBlockID:              DecodeHex(peerInfo.HeadBlockID),
//		HeadBlockTime:            peerInfo.HeadBlockTime.Time,
//		LastIrreversibleBlockNum: uint32(blockInfo.BlockNum),
//		LastIrreversibleBlockID:  DecodeHex(blockInfo.ID),
//	}
//
//	return
//
//}

type HandshakeInfo struct {
	HeadBlockNum             uint32
	HeadBlockID              eos.SHA256Bytes
	HeadBlockTime            time.Time
	LastIrreversibleBlockNum uint32
	LastIrreversibleBlockID  eos.SHA256Bytes
}

func (c *Client) SendHandshake(info *HandshakeInfo) (err error) {
	pulbicKey, err := ecc.NewPublicKey("EOS1111111111111111111111111111111114T1Anm")
	if err != nil {
		return
	}
	signature, err := ecc.NewSignature("SIG_K1_K2WBNtiTY8o4mqFSz7HPnjkiT9JhUYGFa81RrzaXr3aWRF1F8qwVfutJXroqiL35ZiHTcvn8gPWGYJDwnKZTCcbAGJy4i9")
	if err != nil {
		return
	}

	tstamp := eos.Tstamp{Time: info.HeadBlockTime}

	fmt.Println("Time from fake: ", tstamp)

	handshake := &eos.HandshakeMessage{
		NetworkVersion:           c.NetworkVersion,
		ChainID:                  c.ChainID,
		NodeID:                   c.NodeID,
		Key:                      pulbicKey,
		Time:                     tstamp,
		Token:                    DecodeHex("0000000000000000000000000000000000000000000000000000000000000000"),
		Signature:                signature,
		P2PAddress:               c.p2pAddress,
		LastIrreversibleBlockNum: info.LastIrreversibleBlockNum,
		LastIrreversibleBlockID:  info.LastIrreversibleBlockID,
		HeadNum:                  info.HeadBlockNum,
		HeadID:                   info.HeadBlockID,
		OS:                       "linux",
		Agent:                    "Charles Billette Agent",
		Generation:               int16(1),
	}

	err = c.sendMessage(handshake)
	return
}

func (c *Client) SendSyncRequest(startBlockNum uint32, endBlockNumber uint32) (err error) {

	fmt.Printf("SendSyncRequest start [%d] end [%d]\n", startBlockNum, endBlockNumber)
	syncRequest := &eos.SyncRequestMessage{
		StartBlock: startBlockNum,
		EndBlock:   endBlockNumber,
	}

	c.sendMessage(syncRequest)

	return
}

func (c *Client) sendMessage(message eos.P2PMessage) (err error) {

	n, _ := message.GetType().Name()
	fmt.Printf("Sending message [%s] to server\n", n)

	envelope := &eos.P2PMessageEnvelope{
		Type:       message.GetType(),
		P2PMessage: message,
	}

	encoder := eos.NewEncoder(c.Conn)
	err = encoder.Encode(envelope)

	return
}

func (c *Client) handleConnection(route *Route, ready chan bool) {

	r := bufio.NewReader(c.Conn)

	ready <- true
	for {

		envelope, err := eos.ReadP2PMessageData(r)
		if err != nil {
			log.Fatal("Handle connection, ", err)
		}

		pp := PostProcessable{
			Route:              route,
			P2PMessageEnvelope: envelope,
		}

		c.handlersLock.Lock()
		for _, handle := range c.handlers {
			handle.Handle(pp)
		}
		c.handlersLock.Unlock()

	}
}
