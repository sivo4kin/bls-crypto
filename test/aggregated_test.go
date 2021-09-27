package test

import (
	"bytes"
	"log"
	"math/big"
	"math/bits"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"github.com/stretchr/testify/require"
)

var (
	msg         = GenRandomBytes(5000)
	privs, pubs = GenRandomKeys(64)
	aggPub      = AggregatePointsOnG2(pubs)
	mks         = AggregateMembershipKeys(privs, pubs, aggPub)
)

func TestPrecompiled_SimpleAggregatedSignatureInSolidity(t *testing.T) {
	sig := Sign(privs[0], msg)
	for i := 1; i < len(pubs); i++ {
		sgn := Sign(privs[i], msg)
		sig = new(bn256.G1).Add(sig, sgn)
	}
	_, err := blsSignatureTest.VerifySignature(owner, aggPub.Marshal(), msg, sig.Marshal())
	require.NoError(t, err)
	backend.Commit()
	verifiedSol, err := blsSignatureTest.Verified(&bind.CallOpts{})
	require.True(t, verifiedSol)
}

func TestPrecompiled_SignAsPointInSolidity(t *testing.T) {
	var data []byte
	data = append(data, pubs[0].Marshal()...)
	dat := make([]byte, 32)
	dat[31] = 42
	data = append(data, dat...)
	sig := Sign(privs[0], data)
	msgPoint := HashToPointByte(pubs[0], 42)
	_, err := blsSignatureTest.VerifySignaturePoint(owner, pubs[0].Marshal(), msgPoint.Marshal(), sig.Marshal())
	require.NoError(t, err)
	backend.Commit()
	verifiedSol, err := blsSignatureTest.Verified(&bind.CallOpts{})
	require.True(t, verifiedSol)
}

func TestPrecompiled_MembershipKeysInSolidity(t *testing.T) {
	msgPoint := HashToPointByte(aggPub, 0)
	_, err := blsSignatureTest.VerifySignaturePoint(owner, aggPub.Marshal(), msgPoint.Marshal(), mks[0].Marshal())
	require.NoError(t, err)
	backend.Commit()
	verifiedSol, err := blsSignatureTest.Verified(&bind.CallOpts{})
	require.True(t, verifiedSol)
}

func TestPrecompiled_AggregatedHashInSolidity(t *testing.T) {
	index := byte(42)
	dataBytes, err := blsSignatureTest.VerifyAggregatedHash(&bind.CallOpts{}, aggPub.Marshal(), big.NewInt(int64(index)))
	require.NoError(t, err)
	res := HashToPointByte(aggPub, index)
	require.Equal(t, 0, bytes.Compare(dataBytes, res.Marshal()))
}

func TestPrecompiled_2of2VerifyAggregatedInSolidity(t *testing.T) {
	s1 := new(big.Int).SetBytes(GenRandomBytes(64))
	p1 := new(bn256.G2).ScalarBaseMult(s1)
	s2 := new(big.Int).SetBytes(GenRandomBytes(64))
	p2 := new(bn256.G2).ScalarBaseMult(s2)
	p := new(bn256.G2).Add(p1, p2)
	mk11 := new(bn256.G1).ScalarMult(HashToPointByte(p, 0), s1)
	mk12 := new(bn256.G1).ScalarMult(HashToPointByte(p, 1), s1)
	mk1 := new(bn256.G1).Add(mk11, mk12)
	mk21 := new(bn256.G1).ScalarMult(HashToPointByte(p, 0), s2)
	mk22 := new(bn256.G1).ScalarMult(HashToPointByte(p, 1), s2)
	mk2 := new(bn256.G1).Add(mk21, mk22)
	sig1 := SignAggregated(s1, msg, p, mk1)
	sig2 := SignAggregated(s2, msg, p, mk2)
	sig := new(bn256.G1).Add(sig1, sig2)
	bitmask := big.NewInt(3)

	_, err := blsSignatureTest.VerifyAggregatedSignature(owner, p.Marshal(), p.Marshal(), msg, sig.Marshal(), bitmask)
	require.NoError(t, err)
	backend.Commit()
	verifiedSol, err := blsSignatureTest.Verified(&bind.CallOpts{})
	require.True(t, verifiedSol)
}

func signAggregatedPartially(bitmask *big.Int) (pub *bn256.G2, sig *bn256.G1) {
	for i := 0; i < len(pubs); i++ {
		if bitmask.Bit(i) != 0 {
			s := SignAggregated(privs[i], msg, aggPub, mks[i])
			if sig == nil {
				sig = s
				pub = pubs[i]
			} else {
				sig = new(bn256.G1).Add(sig, s)
				pub = new(bn256.G2).Add(pub, pubs[i])
			}
		}
	}
	return
}

func internalTest_KofNVerifyAggregatedInSolidity(t *testing.T, mask int64) {
	// bitmask := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
	bitmask := big.NewInt(mask)
	pub, sig := signAggregatedPartially(bitmask)
	tx, err := blsSignatureTest.VerifyAggregatedSignature(owner, aggPub.Marshal(), pub.Marshal(), msg, sig.Marshal(), bitmask)
	require.NoError(t, err)
	log.Printf("Signers: %3d/%d, gas: %d", bits.OnesCount64(uint64(mask)), len(pubs), tx.Gas())
	backend.Commit()
	verifiedSol, err := blsSignatureTest.Verified(&bind.CallOpts{})
	require.True(t, verifiedSol)
	require.True(t, VerifyAggregated(aggPub, pub, msg, sig, bitmask))
}

func TestPrecompiled_KofNVerifyAggregatedInSolidity(t *testing.T) {
	internalTest_KofNVerifyAggregatedInSolidity(t, 0x7FFFFFFFFFFFFFFF)

	internalTest_KofNVerifyAggregatedInSolidity(t, 0xFFFFFFFF)
	internalTest_KofNVerifyAggregatedInSolidity(t, 0xF0F0F0F0)
	internalTest_KofNVerifyAggregatedInSolidity(t, 0x0F0F0F0F)
	internalTest_KofNVerifyAggregatedInSolidity(t, 0x11111111)
	internalTest_KofNVerifyAggregatedInSolidity(t, 0x10101010)
	internalTest_KofNVerifyAggregatedInSolidity(t, 0x80000001)
	internalTest_KofNVerifyAggregatedInSolidity(t, 1)
}