package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"phqb.com/gethplayground/erc20"
)

const RPC_ENDPOINT = "<ENDPOINT>"

func fetchERC20TransferEvents() {
	var (
		client, _           = ethclient.Dial(RPC_ENDPOINT)
		erc20ABI, _         = erc20.Erc20MetaData.GetAbi()
		erc20ABIInstance, _ = erc20.NewErc20(common.Address{}, client)
		blockHash           = common.HexToHash("0x7008451b87e4f126e3b5428d4ea2c6f23167ddbb8a1c1fa1d4e1d9ca70faaca8")
	)
	erc20TransferLogs, err := client.FilterLogs(context.TODO(), ethereum.FilterQuery{
		Topics:    [][]common.Hash{{erc20ABI.Events["Transfer"].ID}},
		BlockHash: &blockHash,
	})
	if err != nil {
		panic(err)
	}
	for _, log := range erc20TransferLogs {
		transferEvent, err := erc20ABIInstance.ParseTransfer(log)
		if err != nil {
			fmt.Printf("could not parse transfer event, error: %s\n", err)
			continue
		}
		fmt.Printf("ERC20 Transfer from=%s to=%s amount=%s\n", transferEvent.From, transferEvent.To, transferEvent.Value)
	}
}

type address = [20]byte

type txCallOpcodeJSON struct {
	CallOps []struct {
		From address `json:"from"`
		Addr string  `json:"addr"`
		Val  string  `json:"val"`
	} `json:"callOps"`
	CtxFrom  address `json:"from"`
	CtxTo    address `json:"to"`
	CtxValue string  `json:"value"`
}

type traceBlockByHashResultJSON struct {
	JSONRPC string             `json:"jsonrpc"`
	ID      int                `json:"id"`
	Result  []txCallOpcodeJSON `json:"result"`
	Error   *string            `json:"error"`
}

type rpcCallJSON struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type tracerParamJSON struct {
	Tracer string `json:"tracer"`
}

func bigIntToAddress(s string) common.Address {
	bn, _ := new(big.Int).SetString(s, 10)
	return common.BytesToAddress(bn.Bytes())
}

func fetchNativeTransfers() {
	var (
		blockHash  = common.HexToHash("0x7008451b87e4f126e3b5428d4ea2c6f23167ddbb8a1c1fa1d4e1d9ca70faaca8")
		httpClient = &http.Client{}
		tracer     = `{
			retVal: {
				callOps: []
			},
			step: function (log, db) {
				if (log.op.toNumber() == 0xF1)
					this.retVal.callOps.push({
						from: log.contract.getAddress(), 
						addr: log.stack.peek(1),
						val: log.stack.peek(2)
					});
			},
			fault: function(log, db) {},
			result: function(ctx, db) {
				this.retVal.from = ctx.from;
				this.retVal.to = ctx.to;
				this.retVal.value = ctx.value;
				return this.retVal;
			}
		}`
	)
	payload, _ := json.Marshal(rpcCallJSON{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "debug_traceBlockByHash",
		Params: []interface{}{
			blockHash.String(),
			tracerParamJSON{
				Tracer: tracer,
			},
		},
	})
	request, err := http.NewRequest("POST", RPC_ENDPOINT, strings.NewReader(string(payload)))
	if err != nil {
		panic(err)
	}
	request.Header.Add("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(body))
	var result traceBlockByHashResultJSON
	err = json.Unmarshal(body, &result)
	if err != nil {
		panic(err)
	}
	for _, t := range result.Result {
		fmt.Printf("native transfer from=%s to=%s amount=%s\n", common.BytesToAddress(t.CtxFrom[:]), common.BytesToAddress(t.CtxTo[:]), t.CtxValue)
		for _, c := range t.CallOps {
			fmt.Printf("native transfer from=%s to=%s amount=%s\n", common.BytesToAddress(c.From[:]), bigIntToAddress(c.Addr), c.Val)
		}
	}
}

func main() {
	fetchERC20TransferEvents()
	fetchNativeTransfers()
}
