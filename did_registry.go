package contracts

import (
	"encoding/json"
	"fmt"

	"github.com/bitxhub/bitxid"
	"github.com/bitxhub/did-method-registry/converter"
	"github.com/meshplus/bitxhub-core/agency"
	"github.com/meshplus/bitxhub-core/boltvm"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/treasersimplifies/cstr"
)

const (
	DIDRegistryKey = "DIDRegistry"
)

// NewDIDManager .
func NewDIDManager() agency.Contract {
	return &DIDManager{}
}

func init() {
	agency.RegisterContractConstructor("did registry", constant.DIDRegistryContractAddr.Address(), NewDIDManager)
}

// DIDInfo is used for return struct.
type DIDInfo struct {
	DID     string        // did name
	DocAddr string        // address where the doc file stored
	DocHash []byte        // hash of the doc file
	Doc     bitxid.DIDDoc // doc content
	Status  string        // status of did
}

// DIDManager .
type DIDManager struct {
	boltvm.Stub
}

func (dm *DIDManager) getDIDRegistry() *DIDRegistry {
	dr := &DIDRegistry{}
	dm.GetObject(DIDRegistryKey, &dr)
	if dr.Registry != nil {
		dr.loadTable(dm.Stub)
	}
	return dr
}

// DIDRegistry represents all things of did registry.
// @SelfID: self Method ID
// @ChildIDs: Method IDs of the child chain
type DIDRegistry struct {
	// boltvm.Stub
	Registry   *bitxid.DIDRegistry
	Initalized bool
	SelfID     bitxid.DID
	ParentID   bitxid.DID // not used
	ChildIDs   []bitxid.DID
}

// if you need to use registry table, you have to manully load it, so do docdb
// returns err if registry is nil
func (dr *DIDRegistry) loadTable(stub boltvm.Stub) error {
	if dr.Registry == nil {
		return fmt.Errorf("registry is nil")
	}
	dr.Registry.Table = &bitxid.KVTable{
		Store: converter.StubToStorage(stub),
	}
	return nil
}

// Init sets up the whole registry,
// caller should be admin.
func (dm *DIDManager) Init(caller string) *boltvm.Response {
	dr := dm.getDIDRegistry()

	callerDID := bitxid.DID(caller)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}

	if dr.Initalized {
		return boltvm.Error(boltvm.DidRegistryAlreadyInitCode, string(boltvm.DidRegistryAlreadyInitMsg))
	}
	s := converter.StubToStorage(dm.Stub)
	r, err := bitxid.NewDIDRegistry(s, dm.Logger(), bitxid.WithDIDAdmin(bitxid.DID(caller)))
	if err != nil {
		msg := fmt.Sprintf("init err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}
	dr.Registry = r
	err = dr.Registry.SetupGenesis()
	if err != nil {
		msg := fmt.Sprintf("init gensis err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}
	dr.SelfID = dr.Registry.GetSelfID()
	dr.Initalized = true

	dm.SetObject(DIDRegistryKey, dr)
	dm.Logger().Info(cstr.Dye("DID Registry init success v1 !", "Green"))
	return boltvm.Success(nil)
}

// GetMethodID gets method id of the registry.
func (dm *DIDManager) GetMethodID() *boltvm.Response {
	dr := dm.getDIDRegistry()

	return boltvm.Success([]byte(dr.SelfID))
}

// SetMethodID sets method id of did registtry,
// caller should be admin.
func (dm *DIDManager) SetMethodID(caller, method string) *boltvm.Response {
	dr := dm.getDIDRegistry()

	if !dr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}
	if !dr.Registry.HasAdmin(callerDID) {
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}
	dr.SelfID = bitxid.DID(method)

	dm.SetObject(DIDRegistryKey, dr)
	return boltvm.Success(nil)
}

// Register anchors infomation for the did.
func (dm *DIDManager) Register(caller string, docAddr string, docHash []byte, sig []byte) *boltvm.Response {
	dr := dm.getDIDRegistry()

	if !dr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}
	if dr.SelfID != bitxid.DID(callerDID.GetMethod()) {
		return boltvm.Error(boltvm.DidNotOnTheChainCode, fmt.Sprintf(string(boltvm.DidNotOnTheChainMsg), string(callerDID), string(dr.SelfID)))
	}

	docAddr, docHash, err := dr.Registry.Register(bitxid.DocOption{
		ID:   bitxid.DID(callerDID),
		Addr: docAddr,
		Hash: docHash,
	})
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	dm.SetObject(DIDRegistryKey, dr)
	return boltvm.Success(nil)
}

// Update updates did infomation.
func (dm *DIDManager) Update(caller string, docAddr string, docHash []byte, sig []byte) *boltvm.Response {
	dr := dm.getDIDRegistry()

	if !dr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}
	if dr.SelfID != bitxid.DID(callerDID.GetMethod()) {
		return boltvm.Error(boltvm.DidNotOnTheChainCode, fmt.Sprintf(string(boltvm.DidNotOnTheChainMsg), string(callerDID), string(dr.SelfID)))
	}

	docAddr, docHash, err := dr.Registry.Update(bitxid.DocOption{
		ID:   bitxid.DID(callerDID),
		Addr: docAddr,
		Hash: docHash,
	})
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	dm.SetObject(DIDRegistryKey, dr)
	return boltvm.Success(nil)
}

// Resolve gets all infomation of the did.
func (dm *DIDManager) Resolve(caller string) *boltvm.Response {
	dr := dm.getDIDRegistry()

	if !dr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)

	item, _, exist, err := dr.Registry.Resolve(callerDID)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	didInfo := DIDInfo{}
	if exist {
		didInfo = DIDInfo{
			DID:     string(item.ID),
			DocAddr: item.DocAddr,
			DocHash: item.DocHash,
			Status:  string(item.Status),
		}
	}
	b, err := bitxid.Struct2Bytes(didInfo)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	return boltvm.Success(b)
}

// Freeze freezes the did in this registry,
// caller should be admin.
func (dm *DIDManager) Freeze(caller, callerToFreeze string, sig []byte) *boltvm.Response {
	dr := dm.getDIDRegistry()

	if !dr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	callerToFreezeDID := bitxid.DID(callerToFreeze)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}
	if !dr.Registry.HasAdmin(callerDID) {
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}

	item, _, _, err := dr.Registry.Resolve(callerToFreezeDID)
	if item.Status == bitxid.Frozen {
		return boltvm.Error(boltvm.DidAlreadyFrozenCode, fmt.Sprintf(string(boltvm.DidAlreadyFrozenMsg), callerToFreeze))
	}

	err = dr.Registry.Freeze(callerToFreezeDID)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	dm.SetObject(DIDRegistryKey, dr)
	return boltvm.Success(nil)
}

// UnFreeze unfreezes the did in the registry,
// caller should be admin.
func (dm *DIDManager) UnFreeze(caller, callerToUnfreeze string, sig []byte) *boltvm.Response {
	dr := dm.getDIDRegistry()

	if !dr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	callerToUnfreezeDID := bitxid.DID(callerToUnfreeze)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}
	if !dr.Registry.HasAdmin(callerDID) {
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}

	item, _, _, err := dr.Registry.Resolve(callerToUnfreezeDID)
	if item.Status != bitxid.Frozen {
		return boltvm.Error(boltvm.DidNotFrozenCode, fmt.Sprintf(string(boltvm.DidNotFrozenMsg), callerToUnfreeze))
	}

	err = dr.Registry.UnFreeze(callerToUnfreezeDID)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	dm.SetObject(DIDRegistryKey, dr)
	return boltvm.Success(nil)
}

// Delete deletes the did,
// caller should be self, admin can not be deleted.
func (dm *DIDManager) Delete(caller, callerToDelete string, sig []byte) *boltvm.Response {
	dr := dm.getDIDRegistry()

	if !dr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	callerToDeleteDID := bitxid.DID(callerToDelete)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}
	if !dr.Registry.HasAdmin(callerDID) {
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}
	if dr.Registry.HasAdmin(callerToDeleteDID) {
		return boltvm.Error(boltvm.DidDeleteAdminErrCode, string(boltvm.DidDeleteAdminErrMsg))
	}

	err := dr.Registry.Delete(callerToDeleteDID)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	dm.SetObject(DIDRegistryKey, dr)
	return boltvm.Success(nil)
}

// isSuperAdmin querys whether caller is the super admin of the registry.
func (dr *DIDRegistry) isSuperAdmin(caller bitxid.DID) bool {
	admins := dr.Registry.GetAdmins()
	return admins[0] == caller
}

// HasAdmin querys whether caller is an admin of the registry.
func (dm *DIDManager) HasAdmin(caller string) *boltvm.Response {
	dr := dm.getDIDRegistry()

	callerDID := bitxid.DID(caller)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}

	res := dr.Registry.HasAdmin(callerDID)
	if res == true {
		return boltvm.Success([]byte("1"))
	}
	return boltvm.Success([]byte("0"))
}

// GetAdmins get admins of the registry.
func (dm *DIDManager) GetAdmins() *boltvm.Response {
	dr := dm.getDIDRegistry()

	admins := dr.Registry.GetAdmins()
	data, err := json.Marshal(admins)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	return boltvm.Success([]byte(data))
}

// AddAdmin add caller to the admin of the registry,
// caller should be admin.
func (dm *DIDManager) AddAdmin(caller string, adminToAdd string) *boltvm.Response {
	dr := dm.getDIDRegistry()

	callerDID := bitxid.DID(caller)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}
	if !dr.isSuperAdmin(callerDID) {
		return boltvm.Error(boltvm.DidCallerNoEnoughPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoEnoughPermissionMsg), string(callerDID)))
	}

	err := dr.Registry.AddAdmin(bitxid.DID(adminToAdd))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	dm.SetObject(DIDRegistryKey, dr)
	return boltvm.Success(nil)
}

// RemoveAdmin remove admin of the registry,
// caller should be super admin, super admin can not rm self.
func (dm *DIDManager) RemoveAdmin(caller string, adminToRm string) *boltvm.Response {
	dr := dm.getDIDRegistry()

	callerDID := bitxid.DID(caller)
	if dm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), dm.Caller(), caller))
	}
	if !dr.isSuperAdmin(callerDID) {
		return boltvm.Error(boltvm.DidCallerNoEnoughPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoEnoughPermissionMsg), string(callerDID)))
	}

	if dr.isSuperAdmin(bitxid.DID(adminToRm)) {
		return boltvm.Error(boltvm.DidRemoveSuperAdminErrCode, string(boltvm.DidRemoveSuperAdminErrMsg))
	}
	err := dr.Registry.RemoveAdmin(bitxid.DID(adminToRm))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	dm.SetObject(DIDRegistryKey, dr)
	return boltvm.Success(nil)
}

func docIDNotMatchDidError(c1 string, c2 string) string {
	return "doc ID(" + c1 + ") not match the did(" + c2 + ")"
}

func didNotOnThisChainError(did string, method string) string {
	return "DID(" + did + ") not on the chain(" + method + ")"
}
