package personaldata

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/iryonetwork/network-poc/client"
	"github.com/iryonetwork/network-poc/config"
	"github.com/iryonetwork/network-poc/storage/ehr"
)

func New(config *config.Config) *Data {
	fname := newName()
	sname := newSurname()
	name := fmt.Sprintf("%s %s", fname, sname)
	config.Name = name
	return &Data{
		config:     config,
		Category:   "openehr::431|persistent|",
		ID:         config.EosAccount,
		Name:       name,
		Timstamp:   time.Now().Format("2006-01-02T15:04:05.999Z"),
		Language:   "en",
		BirthDate:  randomDate(),
		Gender:     getGender(),
		FirstName:  fname,
		FamilyName: sname,
	}
}

func randomDate() string {
	min := time.Date(1930, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	max := time.Date(2018, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	delta := max - min

	sec, err := rand.Int(rand.Reader, big.NewInt(int64(delta)))
	if err != nil {
		panic(err)
	}
	return time.Unix(sec.Int64()+min, 0).Format("2006-01-02")

}

func getGender() string {
	if time.Now().Unix()%2 == 0 {
		return "local::at0310|Male|"
	}
	return "local::at0311|Female|"
}

func (d *Data) Upload(ehr *ehr.Storage, client *client.Client) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	id, err := ehr.Encrypt(d.config.EosAccount, data, d.config.EncryptionKeys[d.config.EosAccount])
	if err != nil {
		return err
	}
	return client.Upload(d.config.EosAccount, id, false)
}
