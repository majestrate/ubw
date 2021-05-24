package swarm

import (
	"strconv"
	"crypto/tls"
	"net"
	"net/url"
	"net/http"
	"bytes"
	"fmt"
	"encoding/hex"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"io"
	"github.com/majestrate/session2/lib/constants"
	"github.com/majestrate/session2/lib/utils"
	"errors"
)


type ServiceNode struct {
	RemoteIP string `json:"public_ip"`
	StoragePort int `json:"storage_port"`
	IdentityKey string `json:"pubkey_ed25519"`
	EncryptionKey string `json:"pubkey_x25519"`
}

func makeFields(keys ...string) map[string]bool {
	val:= make(map[string]bool)
	for _, key := range keys {
		val[key] = true
	}
	return val
}

func (node *ServiceNode) RPCURL() *url.URL {
	return node.URL("/json_rpc")
}

func (node *ServiceNode) StorageURL() *url.URL {
	return node.URL("/storage_rpc/v1")
}

func (node *ServiceNode) TLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
	}
}

func (node *ServiceNode) StorageAPI(method string, params map[string]interface{}) (result map[string]interface{}, err error) {
	jsonReq := map[string]interface{} {
		"jsonrpc": "2.0",
			"id": 0,
			"method": method,
			"params": params,
		}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(jsonReq)

	client  := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: node.TLSConfig(),
		},
	}

	resp, err := client.Post(node.StorageURL().String(), "application/json", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody := new(bytes.Buffer)
	_, err = io.Copy(responseBody, resp.Body)
	if err != nil {
		return nil, err
	}
	
	jsonResponse := make(map[string]interface{})

	err = json.NewDecoder(responseBody).Decode(&jsonResponse)
	if err != nil {
		return nil, err
	}
	return jsonResponse, nil
}

var zb32 = base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(-1)

func (node *ServiceNode) SNodeAddr() string {
	if node.IdentityKey == "" {
		return node.RemoteIP
	}
	data, _ := hex.DecodeString(node.IdentityKey)
	return zb32.EncodeToString(data) +".snode"
}

func (node *ServiceNode) URL(path string) *url.URL {
	return &url.URL{
		Scheme: "https",
		Host: net.JoinHostPort(node.SNodeAddr(), fmt.Sprintf("%d", node.StoragePort)),
		Path: path,
	}
}

func (node *ServiceNode) StoreMessage(sessionID string, body string) (*ServiceNode, error) {
	fmt.Printf("store for %s at %s\n", sessionID, node.StorageURL())
	request := map[string]interface{} {
		"pubKey": sessionID,
			"ttl": fmt.Sprintf("%d", constants.TTL),
			"timestamp": fmt.Sprintf("%d",utils.TimeNow()),
			"data": base64.StdEncoding.EncodeToString([]byte(body)),
		}
	result, err := node.StorageAPI("store", request)
	if err == nil {
		snodes, ok := result["snodes"]
		if !ok {
			return node, nil
		}
		snode_list := snodes.([]interface{})
		for _, snode_info := range snode_list {
			snode, ok := snode_info.(map[string]interface{})
			if ! ok {
				continue
			}
			port, err := strconv.Atoi(fmt.Sprintf("%s", snode["port"]))
			if err != nil {
				continue
			}
			info := &ServiceNode{
				RemoteIP: fmt.Sprintf("%s", snode["ip"]),
				StoragePort: port,
				IdentityKey: fmt.Sprintf("%s", snode["pubkey_ed25519"]),
				EncryptionKey: fmt.Sprintf("%s", snode["pubkey_x25519"]),
			}
			fmt.Printf("retry via %s\n", info.StorageURL())
			_, err = info.StoreMessage(sessionID, body)
			if err == nil {
				return info, nil
			}	
		}
		err = errors.New("could not store")
	}
	return nil, err
}

func (node *ServiceNode) FetchMessages(sessionID string) ([]string, error){
	request := map[string]interface{} {
		"pubKey": sessionID,
			"lastHash": "",
		}
	result, err := node.StorageAPI("retrieve", request)
	if err != nil {
		return nil, err
	}
	var messages []string
	msgs, ok := result["messages"]
	if !ok {
		return nil, errors.New("invalid data")
	}
	list, ok := msgs.([]interface{})
	if !ok {
		return nil, errors.New("invalid data")
	}
	for _, msg := range list {
		m, ok := msg.(map[string]interface{})
		if !ok {
			return nil, errors.New("invalid data")
		}
		data, err := base64.StdEncoding.DecodeString(fmt.Sprintf("%s", m["data"]))
		if err != nil {
			return nil, err
		}
		messages = append(messages, string(data))
	}
	return messages, nil
}

type serviceNodeResult struct {
	Nodes []ServiceNode `json:"service_node_states"`
}

type serviceNodeListResponse struct {
	Result serviceNodeResult `json:"result"`
}

/// GetSNodeList fetches from this service node a list of all known service nodes
func (node *ServiceNode) GetSNodeList() ([]ServiceNode, error) {
	
	jsonBody := map[string]interface{} {
		"active_only": true,
			"fields": makeFields("public_ip", "storage_port", "pubkey_ed25519", "pubkey_x25519"),
		}
	jsonReq := map[string]interface{} {
		"jsonrpc": "2.0",
		"id": 0,
		"method": "get_n_service_nodes",
		"params": jsonBody,
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(jsonReq)
	
	resp, err := http.Post(node.RPCURL().String(), "application/json", body)

	if err != nil {
		return nil, err
	}

	var response = serviceNodeListResponse{}
	
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	
	return response.Result.Nodes, nil
}
