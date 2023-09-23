package builder

import (
	"bitcoin/address"
	"bitcoin/constant"
	"bitcoin/formating"
	"bitcoin/scripts"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
)

// tweek bool, scripts []interface{}
type BitcoinSignerCallBack func(trDigest []byte, utxo UtxoWithOwner, multiSigPublicKey string) (string, error)

type BitcoinTransactionBuilder struct {
	// transaction outputs
	OutPuts []BitcoinOutputDetails
	/*
		Transaction fee
		Ensure that you have accurately calculated the amounts.
		If the sum of the outputs, including the transaction fee,
		does not match the total amount of UTXOs,
		it will result in an error. Please double-check your calculations.
	*/
	FEE *big.Int
	// network (address.BitcoinNetwork ,ddress.TestnetNetwork)
	Network address.NetworkInfo
	// UTXOS
	Utxos []UtxoWithOwner
	// Trnsaction memo
	Memo string
	/*
		RBF, or Replace-By-Fee, is a feature in Bitcoin that allows you to increase the fee of an unconfirmed
		transaction that you've broadcasted to the network.
		This feature is useful when you want to speed up a
		transaction that is taking longer than expected to get confirmed due to low transaction fees.
	*/
	EnableRBF bool
}

func NewBitcoinTransactionBuilder(spenders []UtxoWithOwner, outPuts []BitcoinOutputDetails, fee *big.Int, network address.NetworkInfo, memo string, enableRBF bool) *BitcoinTransactionBuilder {
	return &BitcoinTransactionBuilder{
		OutPuts:   outPuts,
		Utxos:     spenders,
		FEE:       fee,
		Memo:      memo,
		Network:   network,
		EnableRBF: enableRBF,
	}
}

func (builder *BitcoinTransactionBuilder) HasSegwit() bool {
	for _, element := range builder.Utxos {
		if element.Utxo.IsSegwit() {
			return true
		}
	}
	return false
}

func (builder *BitcoinTransactionBuilder) HasTaproot() bool {
	for _, element := range builder.Utxos {
		if element.Utxo.IsP2tr() {
			return true
		}
	}
	return false
}

// It is used to make the appropriate scriptSig
func buildInputScriptPubKeys(utxo UtxoWithOwner, isTaproot bool) (scripts.Script, error) {
	if utxo.IsMultiSig() {
		script := scripts.ScriptFromRaw(utxo.OwnerDetails.MultiSigAddress.ScriptDetails, true)

		switch utxo.OwnerDetails.MultiSigAddress.Address.GetType() {
		case address.P2WSHInP2SH:
			if isTaproot {
				return utxo.OwnerDetails.MultiSigAddress.Address.ToScriptPubKey(), nil
			}
			return *script, nil
		case address.P2WSH:
			if isTaproot {
				return utxo.OwnerDetails.MultiSigAddress.Address.ToScriptPubKey(), nil
			}
			return *script, nil
		default:
			return scripts.Script{}, errors.New("invalid script type")
		}
	}

	senderPub := utxo.Public()
	switch utxo.Utxo.scriptType {
	case address.P2PK:
		return senderPub.ToRedeemScript(), nil
	case address.P2WSH:
		if isTaproot {
			return senderPub.ToP2WSHAddress().ToScriptPubKey(), nil
		}
		return senderPub.ToP2WSHScript(), nil
	case address.P2PKH:
		return senderPub.ToAddress().ToScriptPubKey(), nil
	case address.P2WPKH:
		if isTaproot {
			return senderPub.ToSegwitAddress().ToScriptPubKey(), nil
		}
		return senderPub.ToAddress().ToScriptPubKey(), nil
	case address.P2TR:
		return senderPub.ToTaprootAddress().ToScriptPubKey(), nil
	case address.P2PKHInP2SH:
		if isTaproot {
			return senderPub.ToP2PKHInP2SH().ToScriptPubKey(), nil
		}
		return senderPub.ToAddress().ToScriptPubKey(), nil
	case address.P2WPKHInP2SH:
		if isTaproot {
			return senderPub.ToP2WPKHInP2SH().ToScriptPubKey(), nil
		}
		return senderPub.ToAddress().ToScriptPubKey(), nil
	case address.P2WSHInP2SH:
		if isTaproot {
			return senderPub.ToP2WSHInP2SH().ToScriptPubKey(), nil
		}
		return senderPub.ToP2WSHScript(), nil
	case address.P2PKInP2SH:
		if isTaproot {
			return senderPub.ToP2PKInP2SH().ToScriptPubKey(), nil
		}
		return senderPub.ToRedeemScript(), nil
	default:
		return scripts.Script{}, errors.New("cannot send from this type of address ")
	}
}

func generateTransactionDigest(scriptPubKeys scripts.Script, input int, utox UtxoWithOwner, transaction scripts.BtcTransaction, taprootAmounts []*big.Int, tapRootPubKeys []scripts.Script) []byte {
	if utox.Utxo.IsSegwit() {
		if utox.Utxo.IsP2tr() {
			return transaction.GetTransactionTaprootDigest(input, tapRootPubKeys, taprootAmounts, 0, scripts.Script{}, constant.LEAF_VERSION_TAPSCRIPT, constant.TAPROOT_SIGHASH_ALL)
		}
		return transaction.GetTransactionSegwitDigit(input, scriptPubKeys, utox.Utxo.value)
	}
	return transaction.GetTransactionDigest(input, scriptPubKeys, constant.SIGHASH_ALL)
}

func buildP2wshOrP2shScriptSig(signedDigest []string, utx UtxoWithOwner) []string {

	return append(append([]string{""}, signedDigest...), utx.OwnerDetails.MultiSigAddress.ScriptDetails)
}

func buildP2shSegwitRedeemScriptSig(utx UtxoWithOwner) ([]string, error) {
	if utx.IsMultiSig() {
		switch utx.OwnerDetails.MultiSigAddress.Address.GetType() {
		case address.P2WSHInP2SH:
			script := scripts.ScriptFromRaw(utx.OwnerDetails.MultiSigAddress.ScriptDetails, true)
			redeem := address.P2WSHAddresssFromScript(*script).ToScriptPubKey()
			return []string{redeem.ToHex()}, nil
		default:
			return nil, errors.New("does not support this script type")
		}
	}
	senderPub := utx.Public()
	switch utx.Utxo.scriptType {
	case address.P2WPKHInP2SH:
		script := senderPub.ToSegwitAddress().ToScriptPubKey()
		return []string{script.ToHex()}, nil
	case address.P2WSHInP2SH:
		script := senderPub.ToP2WSHAddress().ToScriptPubKey()
		return []string{script.ToHex()}, nil
	default:
		return nil, errors.New("does not support this script type")
	}
}

/*
Unlocking Script (scriptSig): The scriptSig is also referred to as
the unlocking script because it provides data and instructions to unlock
a specific output. It contains information and cryptographic signatures
that demonstrate the right to spend the bitcoins associated with the corresponding scriptPubKey output.
*/
func buildScriptSig(signedDigest string, utx UtxoWithOwner) ([]string, error) {
	senderPub := utx.Public()
	if utx.Utxo.IsSegwit() {
		if utx.Utxo.IsP2tr() {
			return []string{signedDigest}, nil
		}

		switch utx.Utxo.scriptType {
		case address.P2WSHInP2SH:
			script := senderPub.ToP2WSHScript()
			return []string{"", signedDigest, script.ToHex()}, nil
		case address.P2WSH:
			script := senderPub.ToP2WSHScript()
			return []string{"", signedDigest, script.ToHex()}, nil
		default:
			return []string{signedDigest, senderPub.ToHex()}, nil
		}
	} else {
		switch utx.Utxo.scriptType {
		case address.P2PK:
			return []string{signedDigest}, nil
		case address.P2PKH:
			return []string{signedDigest, senderPub.ToHex()}, nil
		case address.P2PKHInP2SH:
			script := senderPub.ToAddress().ToScriptPubKey()
			return []string{signedDigest, senderPub.ToHex(), script.ToHex()}, nil
		case address.P2PKInP2SH:
			script := senderPub.ToRedeemScript()
			return []string{signedDigest, script.ToHex()}, nil
		default:
			return nil, errors.New("does not support this script type")
		}
	}
}
func (build *BitcoinTransactionBuilder) buildInputs() []scripts.TxInput {
	var sequance []byte
	if build.EnableRBF {
		sequance = scripts.NewSequence(constant.TYPE_REPLACE_BY_FEE, 0, true).ForInputSequence()
	}
	inputs := make([]scripts.TxInput, len(build.Utxos))
	for i, e := range build.Utxos {
		inputs[i] = *scripts.NewTxInput(e.Utxo.txHash, e.Utxo.vout)
		if i == 0 && build.EnableRBF {
			inputs[i].Sequence = sequance
		}
	}
	return inputs
}

func (build *BitcoinTransactionBuilder) buildOutputs() []scripts.TxOutput {
	outputs := make([]scripts.TxOutput, len(build.OutPuts))
	for i, e := range build.OutPuts {
		outputs[i] = *scripts.NewTxOutput(e.Value, buildOutputScriptPubKey(e))

	}
	return outputs
}

/*
the scriptPubKey of a UTXO (Unspent Transaction Output) is used as the locking
script that defines the spending conditions for the bitcoins associated
with that UTXO. When creating a Bitcoin transaction, the spending conditions
specified by the scriptPubKey must be satisfied by the corresponding scriptSig
in the transaction input to spend the UTXO.
*/
func buildOutputScriptPubKey(addr BitcoinOutputDetails) scripts.Script {
	return addr.Address.ToScriptPubKey()
}

/*
The primary use case for OP_RETURN is data storage. You can embed various types of
data within the OP_RETURN output, such as text messages, document hashes, or metadata
related to a transaction. This data is permanently recorded on the blockchain and can
be retrieved by anyone who examines the blockchain's history.
*/
func opReturn(message string) scripts.Script {
	if _, err := hex.DecodeString(message); err == nil {
		return *scripts.NewScript("OP_RETURN", message)
	}
	toBytes := []byte(message)
	toHex := hex.EncodeToString(toBytes)
	return *scripts.NewScript("OP_RETURN", toHex)
}

// Total amount to spend excluding fees
func (build *BitcoinTransactionBuilder) sumAmounts() *big.Int {
	sum := big.NewInt(0)
	for _, element := range build.OutPuts {
		sum.Add(sum, element.Value)
	}
	return sum
}

// Total amout of all UTXOs
func (build *BitcoinTransactionBuilder) sumUtxoAmount() *big.Int {
	sum := big.NewInt(0)
	for _, element := range build.Utxos {
		sum.Add(sum, element.Utxo.value)
	}
	return sum
}
func (build *BitcoinTransactionBuilder) BuildTransaction(sign BitcoinSignerCallBack) (*scripts.BtcTransaction, error) {
	// build inputs
	txIn := build.buildInputs()
	// build outout
	txOut := build.buildOutputs()
	// check transaction is segwit
	hasSegwit := build.HasSegwit()
	// check transaction is taproot
	hasTaproot := build.HasTaproot()

	// check if you set memos or not
	if !strings.EqualFold(build.Memo, "") {
		txOut = append(txOut, scripts.TxOutput{
			Amount:       big.NewInt(0),
			ScriptPubKey: opReturn(build.Memo),
		})
	}
	// sum of amounts you filled in outputs
	sumAmounts := build.sumAmounts()
	// sum of UTXOS amount
	sumUtxoAmount := build.sumUtxoAmount()
	// sum of outputs amount + transcation fee
	sumAmountsWithFee := new(big.Int).Add(sumAmounts, build.FEE)

	// We will check whether you have spent the correct amounts or not
	if sumAmountsWithFee.Cmp(sumUtxoAmount) != 0 && sign != nil {
		// return nil, errors.New("sum value of utxo not spending")
	}

	// create new transaction with inputs and outputs and isSegwit transaction or not
	transaction := scripts.NewBtcTransaction(txIn, txOut, hasSegwit)
	// we define empty witnesses. maybe the transaction is segwit and We need this
	wintnesses := make([]scripts.TxWitnessInput, 0)

	// when the transaction is taproot and we must use getTaproot tansaction digest
	// we need all of inputs amounts and owner script pub keys
	taprootAmounts := make([]*big.Int, 0)
	taprootScripts := make([]scripts.Script, 0)
	if hasTaproot {
		taprootAmounts = make([]*big.Int, len(build.Utxos))
		taprootScripts = make([]scripts.Script, len(build.Utxos))
		for i, e := range build.Utxos {
			taprootAmounts[i] = e.Utxo.value
			script, err := buildInputScriptPubKeys(e, true)
			if err != nil {
				return nil, err
			}
			taprootScripts[i] = script
		}
	}
	// Well, now let's do what we want for each input
	for i := 0; i < len(txIn); i++ {
		// We receive the owner's ScriptPubKey
		script, err := buildInputScriptPubKeys(build.Utxos[i], false)
		if err != nil {
			return nil, err
		}
		// We generate transaction digest for current input
		digest := generateTransactionDigest(
			script, i, build.Utxos[i], *transaction,
			taprootAmounts, taprootScripts,
		)
		// handle multisig address
		if build.Utxos[i].IsMultiSig() {
			multiSigAddress := build.Utxos[i].OwnerDetails.MultiSigAddress
			sumMultiSigWeight := int(0)
			var mutlsiSigSignatures []string
			for ownerIndex := 0; ownerIndex < len(multiSigAddress.Signers); ownerIndex++ {
				// now we need sign the transaction digest
				sig, err := sign(digest, build.Utxos[i], multiSigAddress.Signers[ownerIndex].PublicKey)
				if err != nil {
					return nil, err
				}
				for weight := 0; weight < multiSigAddress.Signers[ownerIndex].Weight; weight++ {
					if len(mutlsiSigSignatures) >= multiSigAddress.Threshold {
						break
					}
					mutlsiSigSignatures = append(mutlsiSigSignatures, sig)
				}
				sumMultiSigWeight += multiSigAddress.Signers[ownerIndex].Weight
				if sumMultiSigWeight >= multiSigAddress.Threshold {
					break
				}
			}
			// ok we signed, now we need unlocking script for this input
			scriptSig := buildP2wshOrP2shScriptSig(mutlsiSigSignatures, build.Utxos[i])
			// Now we need to add it to the transaction
			// check if current utxo is segwit or not
			wintnesses = append(wintnesses, *scripts.NewTxWitnessInput(scriptSig))
			/*
				check if we need redeemScriptSig or not
				In a Pay-to-Script-Hash (P2SH) Segregated Witness (SegWit) input,
				the redeemScriptSig is needed for historical and compatibility reasons,
				even though the actual script execution has moved to the witness field (the witnessScript).
				This design choice preserves backward compatibility with older Bitcoin clients that do not support SegWit.
			*/
			if build.Utxos[i].Utxo.IsP2shSegwit() {
				p2shSegwitScript, _ := buildP2shSegwitRedeemScriptSig(build.Utxos[i])
				transaction.SetScriptSig(i, *scripts.NewScript(formating.ToInterfaceSlice(p2shSegwitScript)...))
			}
			continue

		}
		// now we need sign the transaction digest
		sig, err := sign(digest, build.Utxos[i], "")
		if err != nil {
			return nil, err
		}
		// ok we signed, now we need unlocking script for this input
		scriptSig, err := buildScriptSig(sig, build.Utxos[i])
		if err != nil {
			return nil, err
		}

		// Now we need to add it to the transaction
		// check if current utxo is segwit or not
		if build.Utxos[i].Utxo.IsSegwit() {
			// ok is segwit and we append to witness list
			wintnesses = append(wintnesses, *scripts.NewTxWitnessInput(scriptSig))
			/*
				check if we need redeemScriptSig or not
				In a Pay-to-Script-Hash (P2SH) Segregated Witness (SegWit) input,
				the redeemScriptSig is needed for historical and compatibility reasons,
				even though the actual script execution has moved to the witness field (the witnessScript).
				This design choice preserves backward compatibility with older Bitcoin clients that do not support SegWit.
			*/
			if build.Utxos[i].Utxo.IsP2shSegwit() {
				p2shSegwitScript, _ := buildP2shSegwitRedeemScriptSig(build.Utxos[i])
				transaction.SetScriptSig(i, *scripts.NewScript(formating.ToInterfaceSlice(p2shSegwitScript)...))
			}
		} else {
			// ok input is not segwit and we use SetScriptSig to set the correct scriptSig
			transaction.SetScriptSig(i, *scripts.NewScript(formating.ToInterfaceSlice(scriptSig)...))
			/*
			 the concept of an "empty witness" is related to Segregated Witness (SegWit) transactions
			 and the way transaction data is structured. When a transaction input is not associated
			 with a SegWit UTXO, it still needs to be compatible with
			 the SegWit transaction format. This is achieved through the use of an "empty witness."
			*/
			if hasSegwit {
				wintnesses = append(wintnesses, scripts.TxWitnessInput{Stack: []string{}})
			}
		}

	}
	// ok we now check if the transaction is segwit We add all witnesses to the transaction
	if hasSegwit {
		transaction.Witnesses = append(transaction.Witnesses, wintnesses...)
	}
	return transaction, nil

}
