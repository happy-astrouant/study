package eos

import (
	"fmt"
	"math"

	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/eoscanada/eos-go/system"

	"github.com/iryonetwork/network-poc/config"
	"github.com/iryonetwork/network-poc/logger"
)

type Storage struct {
	config *config.Config
	api    *eos.API
	log    *logger.Log
}

// New sets the connection to nodeos API up and adds keybag signer
func New(cfg *config.Config, log *logger.Log) (*Storage, error) {
	log.Debugf("Adding API from %s", cfg.EosAPI)
	node := eos.New(cfg.EosAPI)
	node.SetSigner(eos.NewKeyBag())
	s := &Storage{config: cfg, api: node}
	s.api = node
	return s, nil
}

// AccessReq contains fields needed in sending access-contract related actions
type AccessReq struct {
	From eos.AccountName `json:"from"`
	To   eos.AccountName `json:"to"`
}

// GrantAccess adds `to` field in `from` contract table
func (s *Storage) GrantAccess(from, to string) error {
	s.log.Debugf("Eos::grantAccess(%s, %s) called", from, to)
	// Give access action
	action := &eos.Action{
		Account: eos.AN(s.config.EosContractName),
		Name:    eos.ActN("give"),
		Authorization: []eos.PermissionLevel{
			{eos.AN(from), eos.PermissionName("active")},
		},
		ActionData: eos.NewActionData(AccessReq{From: eos.AN(from), To: eos.AN(to)}),
	}
	_, err := s.api.SignPushActions(action)
	return err
}

// RevokeAccess removes `to` field in `from` contract table
func (s *Storage) RevokeAccess(to, from string) error {
	s.log.Debugf("Eos::grantAccess(%s, %s) called", from, to)
	// Remove access action
	action := &eos.Action{
		Account: eos.AN(s.config.EosContractName),
		Name:    eos.ActN("premove"),
		Authorization: []eos.PermissionLevel{
			{eos.AN(from), eos.PermissionName("active")},
		},
		ActionData: eos.NewActionData(AccessReq{From: eos.AN(from), To: eos.AN(to)}),
	}
	_, err := s.api.SignPushActions(action)
	return err
}

// AccessGranted checks if connection between `from` and `to` is establisehd
// Due to uint32 limitations this functions allows connection for up to 4294967295 doctors to a single client
func (s *Storage) AccessGranted(from, to string) (bool, error) {
	s.log.Printf("Eos::accessGranted(%s, %s) called", from, to)
	// Get the table
	r, err := s.api.GetTableRows(eos.GetTableRowsRequest{JSON: true, Scope: to, Code: s.config.EosContractName, Table: "status", Limit: math.MaxUint32})

	s.log.Debugf("Got table: %s", r)
	a := make([]map[string]string, 0)
	r.JSONToStructs(&a)
	// Check if `to` has its field in the table
	b := false
	for _, st := range a {
		for _, n := range st {
			if n == to {
				b = true
			}
		}
	}
	return b, err
}

// DeployContract pushes contract located in contract/eos to blockchain under name specified in config
func (s *Storage) DeployContract() error {
	s.log.Printf("Eos::deployContract() called")

	if s.config.EosContractName == "" {
		return fmt.Errorf("No config.EosContractName specified, unable to deploy contract")
	}

	err := s.pushContract("iryo", "iryo")
	if err != nil {
		return fmt.Errorf("Failed to deploy connections contract: %v", err)
	}
	err = s.pushContract("iryo.token", "eosio.token")
	if err != nil {
		return fmt.Errorf("Failed to deploy new token contract: %v", err)
	}

	return nil
}

func (s *Storage) pushContract(n, cn string) error {
	s.CreateAccount(n)

	// Get newcontract actions
	// TODO: Are the paths ok?
	contract, err := system.NewSetContract(eos.AN(n), "../../contract/eos/"+cn+".wasm", "../../contract/eos/"+cn+".abi")
	if err != nil {
		return err
	}
	for _, a := range contract {
		_, err := s.api.SignPushActions(a)
		if err != nil {
			return err
		}
	}

	return nil
}

// CreateAccount creates account using key specified in config.EosPrivate.
// The key is imported, then account is created
func (s *Storage) CreateAccount(account string) error {
	s.log.Printf("Eos::createAccount() called")

	key, err := ecc.NewPrivateKey(s.config.EosPrivate)
	if err != nil {
		return err
	}
	s.api.Signer.ImportPrivateKey(s.config.EosPrivate)
	// Create new account
	action := system.NewNewAccount(eos.AN("master"), eos.AN(account), key.PublicKey())
	res, err := s.api.SignPushActions(action)
	s.log.Printf("newAccount action response: %#v", res)
	if err != nil {
		s.log.Printf("%s", err)
		return err
	}
	return nil
}