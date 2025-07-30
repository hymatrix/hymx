package schema

const (
	RdbProcessNoncePrefix      = "p:nonce:"  // key: processid, value: nonce
	RdbProcessMsgsPrefix       = "p:msgs:"   // key: processid, value: msg list
	RdbProcessAssignmentPrefix = "p:assign:" // key: processid, value: assignment list

	RdbMsgIndex         = "m:index"    // key: msgid, value: processid + nonce
	RdbMsgResult        = "m:result"   // key: msgid, value: result
	RdbMsgResultsPrefix = "m:results:" // key: processid, value: ordered list of msgid

	RdbCheckpointIndexPrefix = "ckp:index:" // key: processid, value: checkpoint arweave itemid

	RdbCachePrefix = "cache:"
)
