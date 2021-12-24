package contracts

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bitxhub/did-method-registry/converter"
	"github.com/meshplus/bitxhub-core/agency"
	"github.com/meshplus/bitxhub-core/boltvm"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
	"github.com/meshplus/bitxid"
	"github.com/mitchellh/go-homedir"
)

const (
	MethodRegistryKey = "MethodRegistry"
)

// MethodInfo is used for return struct.
// TDDO: rm to pb.
type MethodInfo struct {
	Method  string           // method name
	Owner   string           // owner of the method, is a did
	DocAddr string           // address where the doc file stored
	DocHash []byte           // hash of the doc file
	Doc     bitxid.MethodDoc // doc content
	Status  string           // status of method
}

// MethodManager .
type MethodManager struct {
	boltvm.Stub
}

func (mm *MethodManager) getMethodRegistry() *MethodRegistry {
	mr := &MethodRegistry{}
	mm.GetObject(MethodRegistryKey, &mr)
	if mr.Registry != nil {
		mr.loadTable(mm.Stub)
	}
	return mr
}

// MethodInterRelaychain records inter-relaychain meta data
// @OutCounter records inter-relaychian ibtp numbers of a destiny chain
type MethodInterRelaychain struct {
	OutCounter map[string]uint64
	// OutMessage map[doubleKey]*pb.IBTP
}

// MethodRegistry represents all things of method registry.
// @SelfID: self Method ID
type MethodRegistry struct {
	Registry    *bitxid.MethodRegistry
	Initalized  bool
	SelfID      bitxid.DID
	ParentID    bitxid.DID
	ChildIDs    []bitxid.DID
	IDConverter map[bitxid.DID]string
}

// if you need to use registry table, you have to manully load it, so do docdb
// returns err if registry is nil
func (mr *MethodRegistry) loadTable(stub boltvm.Stub) error {
	if mr.Registry == nil {
		return fmt.Errorf("registry is nil")
	}
	mr.Registry.Table = &bitxid.KVTable{
		Store: converter.StubToStorage(stub),
	}
	mr.Registry.Docdb = &bitxid.KVDocDB{
		Store:     converter.StubToStorage(stub),
		BasicAddr: ".",
	}
	return nil
}

// NewMethodManager .
func NewMethodManager() agency.Contract {
	return &MethodManager{}
}

func init() {
	agency.RegisterContractConstructor("method registry", constant.MethodRegistryContractAddr.Address(), NewMethodManager)
}

// Init sets up the whole registry,
// caller will be admin of the registry.
func (mm *MethodManager) Init(caller string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryAlreadyInitCode, string(boltvm.DidRegistryAlreadyInitMsg))
	}
	s := converter.StubToStorage(mm.Stub)
	r, err := bitxid.NewMethodRegistry(s, mm.Logger(), bitxid.WithMethodAdmin(callerDID))
	if err != nil {
		msg := fmt.Sprintf("init err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}

	mr.Registry = r
	err = mr.Registry.SetupGenesis()
	if err != nil {
		msg := fmt.Sprintf("init genesis err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}
	mr.SelfID = mr.Registry.GetSelfID()
	mr.ParentID = "did:bitxhub:relayroot:." // default parent
	mr.Initalized = true
	mr.IDConverter = make(map[bitxid.DID]string)
	mm.Logger().Info("Method Registry init success with admin: " + string(callerDID))

	mm.SetObject(MethodRegistryKey, mr)

	return boltvm.Success(nil)
}

// SetParent sets parent for the registry
// caller should be admin.
func (mm *MethodManager) SetParent(caller, parentID string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}
	mr.ParentID = bitxid.DID(parentID)

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// AddChild adds child for the registry
// caller should be admin.
func (mm *MethodManager) AddChild(caller, childID string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}

	mr.ChildIDs = append(mr.ChildIDs, bitxid.DID(childID))

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// RemoveChild removes child for the registry
// caller should be admin.
func (mm *MethodManager) RemoveChild(caller, childID string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}

	for i, child := range mr.ChildIDs {
		if child == bitxid.DID(childID) {
			mr.ChildIDs = append(mr.ChildIDs[:i], mr.ChildIDs[i:]...)
		}
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

func (mr *MethodRegistry) setConvertMap(method string, appID string) {
	mr.IDConverter[bitxid.DID(method)] = appID
}

func (mr *MethodRegistry) getConvertMap(method string) string {
	return mr.IDConverter[bitxid.DID(method)]
}

// SetConvertMap .
// caller should be admin.
func (mm *MethodManager) SetConvertMap(caller, method string, appID string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}

	mr.setConvertMap(method, appID)

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// GetConvertMap .
func (mm *MethodManager) GetConvertMap(caller, method string) *boltvm.Response {
	mr := mm.getMethodRegistry()
	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	return boltvm.Success([]byte(mr.getConvertMap(method)))
}

// Apply applys for a method name.
func (mm *MethodManager) Apply(caller, method string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}

	methodDID := bitxid.DID(method)
	if !methodDID.IsValidFormat() {
		return boltvm.Error(boltvm.DidMethodNotValidCode, fmt.Sprintf(string(boltvm.DidMethodNotValidMsg), string(methodDID)))
	}
	err := mr.Registry.Apply(callerDID, bitxid.DID(method)) // success
	if err != nil {
		msg := fmt.Sprintf("apply err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// AuditApply audits apply-request by others,
// caller should be admin.
func (mm *MethodManager) AuditApply(caller, method string, result int32, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}

	var res bool
	if result >= 1 {
		res = true
	} else {
		res = false
	}
	// TODO: verify sig
	err := mr.Registry.AuditApply(bitxid.DID(method), res)
	if err != nil {
		msg := fmt.Sprintf("audit apply err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// Audit audits arbitrary status of the method,
// caller should be admin.
func (mm *MethodManager) Audit(caller, method string, status string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}

	if !mr.Registry.HasAdmin(callerDID) {
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}
	err := mr.Registry.Audit(bitxid.DID(method), bitxid.StatusType(status))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// RegisterWithDoc anchors information for the method.
func (mm *MethodManager) RegisterWithDoc(caller, method string, docByte []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}

	item, _, check, err := mr.Registry.Resolve(bitxid.DID(method))
	if !check {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	if item.Owner != callerDID {
		return boltvm.Error(boltvm.DidMethodNotBelongCode, fmt.Sprintf(string(boltvm.DidMethodNotBelongMsg), method, caller))
	}

	methodDoc := &bitxid.MethodDoc{}
	if err := json.Unmarshal(docByte, methodDoc); err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	_, _, err = mr.Registry.Register(bitxid.DocOption{
		ID:      bitxid.DID(method),
		Content: methodDoc,
	})
	if err != nil {
		msg := fmt.Sprintf("register err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}

	//check doc is saved
	_, _, _, err = mr.Registry.Resolve(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// ResolveWithDoc get method doc for the method in this registry
func (mm *MethodManager) ResolveWithDoc(method string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	_, doc, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	docByte, err := json.Marshal(doc)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	return boltvm.Success(docByte)
}

// Register anchors information for the method.
func (mm *MethodManager) Register(caller, method string, docAddr string, docHash []byte, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	if item.Owner != callerDID {
		return boltvm.Error(boltvm.DidMethodNotBelongCode, fmt.Sprintf(string(boltvm.DidMethodNotBelongMsg), method, caller))
	}
	// TODO: verify sig
	_, _, err = mr.Registry.Register(bitxid.DocOption{
		ID:   bitxid.DID(method),
		Addr: docAddr,
		Hash: docHash,
	})
	if err != nil {
		msg := fmt.Sprintf("register err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}

	item, _, _, err = mr.Registry.Resolve(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)

	/*data, err := bitxid.Struct2Bytes(item)

	// ibtp without index
	ibtps, err := mr.constructIBTPs(
		string(constant.MethodRegistryContractAddr),
		"Synchronize",
		string(mr.SelfID),
		func(toDIDs []bitxid.DID) []string {
			var tos []string
			for _, to := range toDIDs {
				tos = append(tos, string(to))
			}
			return tos
		}(mr.ChildIDs),
		data,
	)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	ibtpsBytes, err := ibtps.Marshal()
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	return mm.CrossInvoke(constant.InterRelayBrokerContractAddr.String(), "RecordIBTPs", pb.Bytes(ibtpsBytes))

	// return boltvm.Success(nil)
	// TODO: construct chain multi sigs
	// return mr.synchronizeOut(string(callerDID), item, [][]byte{[]byte(".")})*/
}

func (mr *MethodRegistry) constructIBTPs(contractID, function, fromMethod string, toMethods []string, data []byte) (*pb.IBTPs, error) {
	content := pb.Content{
		Func: function,
		Args: [][]byte{[]byte(fromMethod), []byte(data)},
	}

	bytes, err := content.Marshal()
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(pb.Payload{
		Encrypted: false,
		Content:   bytes,
	})
	if err != nil {
		return nil, err
	}

	from := mr.getConvertMap(fromMethod)

	var ibtps []*pb.IBTP
	for _, toMethod := range toMethods {
		to := toMethod //
		ibtps = append(ibtps, &pb.IBTP{
			From:    from,
			To:      to,
			Type:    pb.IBTP_INTERCHAIN,
			Proof:   []byte("1"),
			Payload: payload,
		})
	}

	return &pb.IBTPs{Ibtps: ibtps}, nil
}

// Event .
type Event struct {
	contractID string
	function   string
	fromMethod string
	data       []byte
	tos        []string
}

// Update updates method infomation.
func (mm *MethodManager) Update(caller, method string, docAddr string, docHash []byte, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if item.Owner != callerDID {
		return boltvm.Error(boltvm.DidMethodNotBelongCode, fmt.Sprintf(string(boltvm.DidMethodNotBelongMsg), method, caller))
	}
	_, _, err = mr.Registry.Update(bitxid.DocOption{
		ID:   bitxid.DID(method),
		Addr: docAddr,
		Hash: docHash,
	})
	if err != nil {
		msg := fmt.Sprintf("update err, %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// Resolve gets all infomation for the method in this registry.
func (mm *MethodManager) Resolve(method string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	item, _, exist, err := mr.Registry.Resolve(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	methodInfo := MethodInfo{}
	if exist {
		methodInfo = MethodInfo{
			Method:  string(item.ID),
			Owner:   string(item.Owner),
			DocAddr: item.DocAddr,
			DocHash: item.DocHash,
			Status:  string(item.Status),
		}
		// content := pb.Content{
		// 	SrcContractId: mr.Callee(),
		// 	DstContractId: mr.Callee(),
		// 	Func:          "Resolve",
		// 	Args:          [][]byte{[]byte(caller), []byte(method), []byte(sig)},
		// 	Callback:      "Synchronize",
		// }
		// bytes, err := content.Marshal()
		// if err != nil {
		// 	return boltvm.Error(err.Error())
		// }
		// payload, err := json.Marshal(pb.Payload{
		// 	Encrypted: false,
		// 	Content:   bytes,
		// })
		// if err != nil {
		// 	return boltvm.Error(err.Error())
		// }
		// ibtp := pb.IBTP{
		// 	From:    mr.IDConverter[mr.SelfID],
		// 	To:      mr.IDConverter[mr.ParentID],
		// 	Payload: payload,
		// 	Proof:   []byte("."), // TODO: add proof
		// }
		// data, err := ibtp.Marshal()
		// if err != nil {
		// 	return boltvm.Error(err.Error())
		// }
		// res := mr.CrossInvoke(constant.InterchainContractAddr.String(), "HandleDID", pb.Bytes(data))
		// if !res.Ok {
		// 	return res
		// }
		// return boltvm.Success([]byte("routing..."))
	}

	b, err := bitxid.Struct2Bytes(methodInfo)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	return boltvm.Success(b)
}

// Freeze freezes the method in the registry,
// caller should be admin.
func (mm *MethodManager) Freeze(caller, method string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if item.Status == bitxid.Frozen {
		return boltvm.Error(boltvm.DidMethodFrozenCode, fmt.Sprintf(string(boltvm.DidMethodFrozenMsg), method))
	}

	err = mr.Registry.Freeze(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// UnFreeze unfreezes the method,
// caller should be admin.
func (mm *MethodManager) UnFreeze(caller, method string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if item.Status != bitxid.Frozen {
		return boltvm.Error(boltvm.DidMethodNotFrozenCode, fmt.Sprintf(string(boltvm.DidMethodNotFrozenMsg), method))
	}

	err = mr.Registry.UnFreeze(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// Delete deletes the method,
// caller should be did who owns the method.
func (mm *MethodManager) Delete(caller, method string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if item.Owner != callerDID {
		return boltvm.Error(boltvm.DidMethodNotBelongCode, fmt.Sprintf(string(boltvm.DidMethodNotBelongMsg), method, caller))
	}

	err = mr.Registry.Delete(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// Synchronize synchronizes registry data between different registrys,
// use ibtp.proof to verify, it should only be called within interchain contract.
// @from: sourcechain method id
func (mm *MethodManager) Synchronize(from string, itemb []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error(boltvm.DidRegistryNotInitCode, string(boltvm.DidRegistryNotInitMsg))
	}

	item := &bitxid.MethodItem{}
	err := bitxid.Bytes2Struct(itemb, item)
	if err != nil {
		msg := fmt.Sprintf("Synchronize err: %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}
	// TODO: verify multi sigs of from chain
	// sigs := [][]byte{}
	// err = bitxid.Bytes2Struct(sigsb, &sigs)
	// if err != nil {
	// 	return boltvm.Error("synchronize err: " + err.Error())
	// }

	err = mr.Registry.Synchronize(item)
	if err != nil {
		msg := fmt.Sprintf("Synchronize err: %s", err.Error())
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), msg))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
	// TODO add receipt proof if callback enabled
}

func (mm *MethodManager) synchronizeOut(from string, item *bitxid.MethodItem, sigs [][]byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	itemBytes, err := bitxid.Struct2Bytes(item)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	sigsBytes, err := bitxid.Struct2Bytes(item)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	content := pb.Content{
		Func: "Synchronize",
		Args: [][]byte{[]byte(from), itemBytes, sigsBytes},
	}
	bytes, err := content.Marshal()
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	payload, err := json.Marshal(pb.Payload{
		Encrypted: false,
		Content:   bytes,
	})
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	fromChainID := mr.IDConverter[mr.SelfID]
	for _, child := range mr.ChildIDs {
		toChainID := mr.IDConverter[child]
		ibtp := pb.IBTP{
			From:    fromChainID,
			To:      toChainID, // TODO
			Payload: payload,
		}
		data, err := ibtp.Marshal()
		if err != nil {
			return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
		}
		res := mm.CrossInvoke(constant.InterchainContractAddr.String(), "HandleDID", pb.Bytes(data))
		if !res.Ok {
			mm.Logger().Error("synchronizeOut err, ", string(res.Result))
			return res
		}
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// IsSuperAdmin querys whether caller is the super admin of the registry.
func (mr *MethodRegistry) isSuperAdmin(caller bitxid.DID) bool {
	admins := mr.Registry.GetAdmins()
	return admins[0] == caller
}

// HasAdmin querys whether caller is an admin of the registry.
func (mm *MethodManager) HasAdmin(caller string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	res := mr.Registry.HasAdmin(bitxid.DID(caller))
	if res == true {
		return boltvm.Success([]byte("1"))
	}
	return boltvm.Success([]byte("0"))
}

// GetAdmins get admin list of the registry.
func (mm *MethodManager) GetAdmins() *boltvm.Response {
	mr := &MethodRegistry{}
	mm.GetObject(MethodRegistryKey, &mr)

	admins := mr.Registry.GetAdmins()
	data, err := json.Marshal(admins)
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}
	return boltvm.Success([]byte(data))
}

// AddAdmin adds caller to the admin of the registry,
// caller should be super admin.
func (mm *MethodManager) AddAdmin(caller string, adminToAdd string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.isSuperAdmin(callerDID) { // require Admin
		return boltvm.Error(boltvm.DidCallerNoEnoughPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoEnoughPermissionMsg), string(callerDID)))
	}

	err := mr.Registry.AddAdmin(bitxid.DID(adminToAdd))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

// RemoveAdmin remove admin of the registry,
// caller should be super admin, super admin can not rm self.
func (mm *MethodManager) RemoveAdmin(caller string, adminToRm string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(boltvm.DidCallerNotMatchCode, fmt.Sprintf(string(boltvm.DidCallerNotMatchMsg), mm.Caller(), caller))
	}
	if !mr.isSuperAdmin(callerDID) { // require super Admin
		return boltvm.Error(boltvm.DidCallerNoEnoughPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoEnoughPermissionMsg), string(callerDID)))
	}
	if !mr.Registry.HasAdmin(bitxid.DID(adminToRm)) {
		return boltvm.Error(boltvm.DidCallerNoPermissionCode, fmt.Sprintf(string(boltvm.DidCallerNoPermissionMsg), string(callerDID)))
	}
	if mr.isSuperAdmin(bitxid.DID(adminToRm)) {
		return boltvm.Error(boltvm.DidRemoveSuperAdminErrCode, string(boltvm.DidRemoveSuperAdminErrMsg))
	}

	err := mr.Registry.RemoveAdmin(bitxid.DID(adminToRm))
	if err != nil {
		return boltvm.Error(boltvm.DidInternalErrCode, fmt.Sprintf(string(boltvm.DidInternalErrMsg), err.Error()))
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
}

func callerNotMatchError(c1 string, c2 string) string {
	return "tx.From(" + c1 + ") and callerDID:(" + c2 + ") not the comply"
}

func methodNotBelongError(method string, caller string) string {
	return "method (" + method + ") not belongs to caller(" + caller + ")"
}

func docIDNotMatchMethodError(c1 string, c2 string) string {
	return "doc ID(" + c1 + ") not match the method(" + c2 + ")"
}

func pathRoot() (string, error) {
	dir := os.Getenv("BITXHUB_PATH")
	var err error
	if len(dir) == 0 {
		dir, err = homedir.Expand("~/.bitxhub")
	}
	return dir, err
}
