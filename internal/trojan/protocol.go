package trojan

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"

	"github.com/yanghuajun/proxy/internal/config"
)

// dialTrojan establishes a complete Trojan connection with TLS and protocol handshake
func dialTrojan(cfg *config.NodeConfig, address string) (net.Conn, error) {
	// Step 1: Create TLS configuration
	tlsConfig := &tls.Config{
		ServerName:         cfg.SSL.SNI,
		InsecureSkipVerify: !cfg.SSL.Verify,
		NextProtos:         cfg.SSL.ALPN,
	}

	// Step 2: Establish TLS connection to Trojan server
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
	tlsConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("TLS dial failed: %w", err)
	}

	// Step 3: Send Trojan protocol request
	if err := sendTrojanRequest(tlsConn, cfg.Password, address); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("trojan handshake failed: %w", err)
	}

	return tlsConn, nil
}

// sendTrojanRequest sends Trojan protocol authentication and target address
func sendTrojanRequest(conn net.Conn, password, address string) error {
	// Trojan Request Format:
	// +-----------------------+---------+----------------+---------+
	// | hex(SHA224(password)) |  CRLF   | Trojan Request |  CRLF   |
	// +-----------------------+---------+----------------+---------+
	// |          56           | X'0D0A' |    Variable    | X'0D0A' |
	// +-----------------------+---------+----------------+---------+
	//
	// Trojan Request:
	// +-----+------+----------+----------+
	// | CMD | ATYP | DST.ADDR | DST.PORT |
	// +-----+------+----------+----------+
	// |  1  |  1   | Variable |    2     |
	// +-----+------+----------+----------+

	// Step 1: Calculate password hash (SHA224)
	hash := sha256.Sum224([]byte(password))
	hexHash := hex.EncodeToString(hash[:])

	// Step 2: Start building request
	request := []byte(hexHash + "\r\n")

	// Step 3: Add command (0x01 = CONNECT)
	request = append(request, 0x01)

	// Step 4: Parse target address
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address %s: %w", address, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port in %s: %w", address, err)
	}

	// Step 5: Add address type and address
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			// IPv4 address
			request = append(request, 0x01)
			request = append(request, ip.To4()...)
		} else {
			// IPv6 address
			request = append(request, 0x04)
			request = append(request, ip.To16()...)
		}
	} else {
		// Domain name
		request = append(request, 0x03)
		if len(host) > 255 {
			return fmt.Errorf("domain name too long: %s", host)
		}
		request = append(request, byte(len(host)))
		request = append(request, []byte(host)...)
	}

	// Step 6: Add port (big-endian)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	request = append(request, portBytes...)

	// Step 7: Add CRLF
	request = append(request, '\r', '\n')

	// Step 8: Send the complete request
	_, err = conn.Write(request)
	if err != nil {
		return fmt.Errorf("failed to write trojan request: %w", err)
	}

	return nil
}
