package server

import (
	"gopkg.in/inconshreveable/log15.v2"
	"fmt"
	"net"
	"bufio"
	"time"
	"strings"
	"errors"
)

type ClientHandler struct {
	Id             string              // Id of the client
	daddy          *FtpServer          // Server on which the connection was accepted
	writer         *bufio.Writer       // Writer on the TCP connection
	reader         *bufio.Reader       // Reader on the TCP connection
	conn           net.Conn            // TCP connection
	user           string              // Authenticated user
	path           string              // Current path
	command        string              // Command received on the connection
	param          string              // Param of the FTP command
	connectedAt    int64               // Date of connection
	lastPassCid    string              // Last Passive connection Id
	userInfo       map[string]string   // Various user information (shared between server and driver)
	debug          bool                // Show debugging info on the server side
	driverInstance interface{}         // Instance of the driver's matching object
	transfer       TransferHandler     // Transfer connection (passive one)
}

func (server *FtpServer) NewClientHandler(connection net.Conn) *ClientHandler {

	p := &ClientHandler{
		daddy: server,
		conn: connection,
		Id: genClientID(),
		writer: bufio.NewWriter(connection),
		reader: bufio.NewReader(connection),
		connectedAt: time.Now().UTC().UnixNano(),
		path: "/",
		userInfo: make(map[string]string),
		debug: true,
	}

	// Just respecting the existing logic here, this could be probably be dropped at some point
	p.userInfo["path"] = p.path

	return p
}

func (p *ClientHandler) Die() {
	p.daddy.driver.UserLeft(p)
	p.conn.Close()
	p.daddy.ClientDeparture(p)
}

func (p *ClientHandler) UserInfo() map[string]string {
	return p.userInfo
}

func (p *ClientHandler) Path() string {
	return p.userInfo["path"]
}

func (p *ClientHandler) SetPath(path string) {
	p.userInfo["path"] = path
}

func (p *ClientHandler) MyInstance() interface{} {
	return p.driverInstance
}

func (p *ClientHandler) SetMyInstance(value interface{}) {
	p.driverInstance = value
}

func (p *ClientHandler) HandleCommands() {
	p.daddy.ClientArrival(p)
	defer p.daddy.ClientDeparture(p)

	//fmt.Println(p.id, " Got client on: ", p.ip)
	if msg, err := p.daddy.driver.WelcomeUser(p); err == nil {
		p.writeMessage(220, msg)
	} else {
		p.writeMessage(500, msg)
		p.Die()
		return
	}

	for {
		line, err := p.reader.ReadString('\n')

		if p.debug {
			log15.Info("FTP RECV", "action", "ftp.cmd_recv", "line", line)
		}

		if err != nil {
			log15.Error("TCP error", "err", err)
			return
		}
		command, param := parseLine(line)
		p.command = command
		p.param = param

		fn := commandsMap[command]
		if fn == nil {
			p.writeMessage(550, "not allowed")
		} else {
			fn(p)
		}
	}
}

func (c *ClientHandler) writeMessage(code int, message string) {
	line := fmt.Sprintf("%d %s\r\n", code, message)
	if c.debug {
		log15.Info("FTP SEND", "action", "ftp.cmd_send", "line", line)
	}
	c.writer.WriteString(line)
	c.writer.Flush()
}

func (c *ClientHandler) TransferOpen() (net.Conn, error) {
	if c.transfer != nil {
		return c.transfer.Open()
	} else {
		return nil, errors.New("No passive connection declared")
	}
}

func (c *ClientHandler) TransferClose() {
	if c.transfer != nil {
		c.transfer.Close()
		c.transfer = nil
	}
}

func parseLine(line string) (string, string) {
	params := strings.SplitN(strings.Trim(line, "\r\n"), " ", 2)
	if len(params) == 1 {
		return params[0], ""
	}
	return params[0], strings.TrimSpace(params[1])
}