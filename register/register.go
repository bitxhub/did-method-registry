package register

import (
	"github.com/meshplus/bitxhub-core/agency"
	"github.com/meshplus/bitxhub-kit/storage"
	"github.com/meshplus/bitxid"
	"github.com/sirupsen/logrus"
)

func NewMethodRegistryRegister(ts storage.Storage, l logrus.FieldLogger) agency.Registry {
	r, err := bitxid.NewMethodRegistry(ts, l)
	if err != nil {
		return nil
	}
	return r
}

func NewDIDRegistryRegister(ts storage.Storage, l logrus.FieldLogger) agency.Registry {
	r, err := bitxid.NewDIDRegistry(ts, l)
	if err != nil {
		return nil
	}
	return r
}

func init() {
	agency.RegisterRegistryConstructor("method", NewMethodRegistryRegister)
	agency.RegisterRegistryConstructor("did", NewDIDRegistryRegister)
}
