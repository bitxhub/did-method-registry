package contracts

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bitxhub/bitxid"
	"github.com/bitxhub/did-method-registry/converter"
	"github.com/meshplus/bitxhub-core/agency"
	"github.com/meshplus/bitxhub-core/boltvm"
	"github.com/meshplus/bitxhub-model/constant"
	"github.com/meshplus/bitxhub-model/pb"
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
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if mr.Initalized {
		return boltvm.Error("init err, already init")
	}
	s := converter.StubToStorage(mm.Stub)
	r, err := bitxid.NewMethodRegistry(s, mm.Logger(), bitxid.WithMethodAdmin(callerDID))
	if err != nil {
		return boltvm.Error("init err, " + err.Error())
	}

	mr.Registry = r
	err = mr.Registry.SetupGenesis()
	if err != nil {
		return boltvm.Error("init genesis err, " + err.Error())
	}
	mr.SelfID = mr.Registry.GetSelfID()
	mr.ParentID = "did:bitxhub:relayroot:." // default parent
	mr.Initalized = true
	mr.IDConverter = make(map[bitxid.DID]string)
	mm.Logger().Info("Method Registry init success with admin: " + string(callerDID))
	mm.Logger().Info("Method Registry Version: AuditApply Changed")

	mm.SetObject(MethodRegistryKey, mr)

	return boltvm.Success(nil)
}

// SetParent sets parent for the registry
// caller should be admin.
func (mm *MethodManager) SetParent(caller, parentID string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error("caller(" + string(callerDID) + ") has no permission")
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
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error("caller(" + string(callerDID) + ") has no permission")
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
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error("caller(" + string(callerDID) + ") has no permission")
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
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error("caller(" + string(callerDID) + ") has no permission")
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
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	return boltvm.Success([]byte(mr.getConvertMap(method)))
}

// Apply applys for a method name.
func (mm *MethodManager) Apply(caller, method string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Apply method:%s, time(start) = %s(us)", method, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}

	methodDID := bitxid.DID(method)
	if !methodDID.IsValidFormat() {
		return boltvm.Error("not valid method format")
	}
	err := mr.Registry.Apply(callerDID, bitxid.DID(method)) // success
	if err != nil {
		return boltvm.Error("apply err, " + err.Error())
	}
	mm.SetObject(MethodRegistryKey, mr)

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Apply method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.Apply method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return boltvm.Success(nil)
}

// AuditApply audits apply-request by others,
// caller should be admin.
func (mm *MethodManager) AuditApply(caller, method string, result int32, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.AuditApply method:%s, time(start) = %s(us)", method, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		// return boltvm.Error("caller(" + string(callerDID) + ") has no permission")
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
		return boltvm.Error("audit apply err, " + err.Error())
	}
	mm.SetObject(MethodRegistryKey, mr)

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.AuditApply method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.AuditApply method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return boltvm.Success(nil)
}

// Audit audits arbitrary status of the method,
// caller should be admin.
func (mm *MethodManager) Audit(caller, method string, status string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Audit method:%s, time(start) = %s(us)", method, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}

	if !mr.Registry.HasAdmin(callerDID) {
		return boltvm.Error("caller(" + string(callerDID) + ") has no permission")
	}
	err := mr.Registry.Audit(bitxid.DID(method), bitxid.StatusType(status))
	if err != nil {
		return boltvm.Error(err.Error())
	}
	mm.SetObject(MethodRegistryKey, mr)

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Audit method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.Audit method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return boltvm.Success(nil)
}

// Register anchors infomation for the method.
func (mm *MethodManager) Register(caller, method string, docAddr string, docHash []byte, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Register method:%s, time(start) = %s(us)", method, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(err.Error())
	}
	if item.Owner != callerDID {
		// return boltvm.Error(methodNotBelongError(method, caller))
	}

	// TODO: verify sig
	_, _, err = mr.Registry.Register(bitxid.DocOption{
		ID:   bitxid.DID(method),
		Addr: docAddr,
		Hash: docHash,
	})
	if err != nil {
		return boltvm.Error("register err, " + err.Error())
	}

	item, _, _, err = mr.Registry.Resolve(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(err.Error())
	}

	mm.SetObject(MethodRegistryKey, mr)
	data, err := bitxid.Struct2Bytes(item)

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
		return boltvm.Error(err.Error())
	}

	ibtpsBytes, err := ibtps.Marshal()
	if err != nil {
		return boltvm.Error(err.Error())
	}

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Register method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.Register method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return mm.CrossInvoke(constant.InterRelayBrokerContractAddr.String(), "RecordIBTPs", pb.Bytes(ibtpsBytes))

	// return boltvm.Success(nil)
	// TODO: construct chain multi sigs
	// return mr.synchronizeOut(string(callerDID), item, [][]byte{[]byte(".")})
}

func (mr *MethodRegistry) constructIBTPs(contractID, function, fromMethod string, toMethods []string, data []byte) (*pb.IBTPs, error) {
	content := pb.Content{
		SrcContractId: contractID,
		DstContractId: contractID,
		Func:          function,
		Args:          [][]byte{[]byte(fromMethod), []byte(data)},
		Callback:      "",
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
			From:      from,
			To:        to,
			Type:      pb.IBTP_INTERCHAIN,
			Timestamp: time.Now().UnixNano(),
			Proof:     []byte("1"),
			Payload:   payload,
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

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Update method:%s, time(start) = %s(us)", caller, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if item.Owner != callerDID {
		// return boltvm.Error(methodNotBelongError(method, caller))
	}
	_, _, err = mr.Registry.Update(bitxid.DocOption{
		ID:   bitxid.DID(method),
		Addr: docAddr,
		Hash: docHash,
	})
	if err != nil {
		return boltvm.Error("update err, " + err.Error())
	}
	mm.SetObject(MethodRegistryKey, mr)

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Update method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.Update method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return boltvm.Success(nil)
}

// Resolve gets all infomation for the method in this registry.
func (mm *MethodManager) Resolve(method string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Resolve method:%s, time(start) = %s(us)", method, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	item, _, exist, err := mr.Registry.Resolve(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(err.Error())
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
		return boltvm.Error(err.Error())
	}

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Resolve method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.Resolve method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return boltvm.Success(b)
}

// Freeze freezes the method in the registry,
// caller should be admin.
func (mm *MethodManager) Freeze(caller, method string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Freeze method:%s, time(start) = %s(us)", method, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		// return boltvm.Error("caller(" + string(callerDID) + ") has no permission")
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if item.Status == bitxid.Frozen {
		// return boltvm.Error(method + " was already frozen")
	}

	err = mr.Registry.Freeze(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(err.Error())
	}
	mm.SetObject(MethodRegistryKey, mr)

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Freeze method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.Freeze method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return boltvm.Success(nil)
}

// UnFreeze unfreezes the method,
// caller should be admin.
func (mm *MethodManager) UnFreeze(caller, method string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.UnFreeze method:%s, time(start) = %s(us)", method, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.Registry.HasAdmin(callerDID) { // require Admin
		return boltvm.Error("caller(" + string(callerDID) + ") has no permission")
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if item.Status != bitxid.Frozen {
		return boltvm.Error(method + " was not frozen")
	}

	err = mr.Registry.UnFreeze(bitxid.DID(method))
	if err != nil {
		return boltvm.Error(err.Error())
	}
	mm.SetObject(MethodRegistryKey, mr)

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.UnFreeze method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.UnFreeze method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return boltvm.Success(nil)
}

// Delete deletes the method,
// caller should be did who owns the method.
func (mm *MethodManager) Delete(caller, method string, sig []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()
	mm.Logger().Info("Method Registry Delete: " + string(method))

	startTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Delete method:%s, time(start) = %s(us)", method, startTimeUs)

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}

	item, _, _, err := mr.Registry.Resolve(bitxid.DID(method))
	if item.Owner != callerDID {
		// return boltvm.Error("caller(" + string(callerDID) + ") is not the owner of " + method)
	}

	// err = mr.Registry.Delete(bitxid.DID(method))
	err = mr.Registry.Delete(bitxid.DID("did:.:.:."))
	if err != nil {
		return boltvm.Error(err.Error())
	}
	mm.SetObject(MethodRegistryKey, mr)

	endTimeUs := time.Now().UnixNano() / 1e3
	mm.Logger().Infof("Method.Delete method:%s, time(end) = %s(us)", method, endTimeUs)
	mm.Logger().Infof("Method.Delete method:%s, time(start - end) = %s(us)", method, endTimeUs-startTimeUs)

	return boltvm.Success(nil)
}

// Synchronize synchronizes registry data between different registrys,
// use ibtp.proof to verify, it should only be called within interchain contract.
// @from: sourcechain method id
func (mm *MethodManager) Synchronize(from string, itemb []byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	if !mr.Initalized {
		return boltvm.Error("Registry not initialized")
	}

	item := &bitxid.MethodItem{}
	err := bitxid.Bytes2Struct(itemb, item)
	if err != nil {
		return boltvm.Error("Synchronize err: " + err.Error())
	}
	// TODO: verify multi sigs of from chain
	// sigs := [][]byte{}
	// err = bitxid.Bytes2Struct(sigsb, &sigs)
	// if err != nil {
	// 	return boltvm.Error("synchronize err: " + err.Error())
	// }

	err = mr.Registry.Synchronize(item)
	if err != nil {
		return boltvm.Error("Synchronize err: " + err.Error())
	}

	mm.SetObject(MethodRegistryKey, mr)
	return boltvm.Success(nil)
	// TODO add receipt proof if callback enabled
}

func (mm *MethodManager) synchronizeOut(from string, item *bitxid.MethodItem, sigs [][]byte) *boltvm.Response {
	mr := mm.getMethodRegistry()

	itemBytes, err := bitxid.Struct2Bytes(item)
	if err != nil {
		return boltvm.Error(err.Error())
	}
	sigsBytes, err := bitxid.Struct2Bytes(item)
	if err != nil {
		return boltvm.Error(err.Error())
	}
	content := pb.Content{
		SrcContractId: mm.Callee(),
		DstContractId: mm.Callee(),
		Func:          "Synchronize",
		Args:          [][]byte{[]byte(from), itemBytes, sigsBytes},
		Callback:      "",
	}
	bytes, err := content.Marshal()
	if err != nil {
		return boltvm.Error(err.Error())
	}
	payload, err := json.Marshal(pb.Payload{
		Encrypted: false,
		Content:   bytes,
	})
	if err != nil {
		return boltvm.Error(err.Error())
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
			return boltvm.Error(err.Error())
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
		return boltvm.Error(err.Error())
	}
	return boltvm.Success([]byte(data))
}

// AddAdmin adds caller to the admin of the registry,
// caller should be super admin.
func (mm *MethodManager) AddAdmin(caller string, adminToAdd string) *boltvm.Response {
	mr := mm.getMethodRegistry()

	callerDID := bitxid.DID(caller)
	if mm.Caller() != callerDID.GetAddress() {
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.isSuperAdmin(callerDID) { // require Admin
		return boltvm.Error("caller(" + string(callerDID) + ") doesn't have enough permission")
	}

	err := mr.Registry.AddAdmin(bitxid.DID(adminToAdd))
	if err != nil {
		return boltvm.Error(err.Error())
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
		return boltvm.Error(callerNotMatchError(mm.Caller(), caller))
	}
	if !mr.isSuperAdmin(callerDID) { // require super Admin
		return boltvm.Error("caller(" + string(callerDID) + ") doesn't have enough permission")
	}
	if !mr.Registry.HasAdmin(bitxid.DID(adminToRm)) {
		return boltvm.Error("caller (" + caller + ") is not admin")
	}
	if mr.isSuperAdmin(bitxid.DID(adminToRm)) {
		return boltvm.Error("cannot rm super admin")
	}

	err := mr.Registry.RemoveAdmin(bitxid.DID(adminToRm))
	if err != nil {
		return boltvm.Error(err.Error())
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
