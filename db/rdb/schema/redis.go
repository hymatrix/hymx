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

// chainkit constants
const (
	RdbPendingTxIds     = "chainkit:pending"              // List: Pending TxId FIFO queue
	RdbUploadingTxIds   = "chainkit:uploading"            // Set: Uploading TxId pool
	RdbCurrentBundledIn = "chainkit:current_bundledin_id" // String: current bundledIn id with 1 hour expiration
	RdbUploadedTxIds    = "chainkit:uploaded_txids"       // Set: Uploaded TxId pool
	RdbChainkitCache    = "chainkit:cache"                // key: TxId, value BundleItem

	// MaxUploadingCount is the maximum number of transactions allowed in uploading state
	MaxUploadingCount = 100000 // 10w
)
