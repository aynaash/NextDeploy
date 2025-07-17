package shared

// import (
// 	"crypto/rand"
// 	"errors"
// 	"fmt"
// )
//
// // RunCryptoHealthChecks verifies all cryptographic operations work correctly
// func RunCryptoHealthChecks() error {
// 	// Test ECDH key exchange
// 	if err := testECDH(); err != nil {
// 		return fmt.Errorf("ECDH health check failed: %v", err)
// 	}
//
// 	// Test AES-GCM encryption
// 	if err := testAESGCM(); err != nil {
// 		return fmt.Errorf("AES-GCM health check failed: %v", err)
// 	}
//
// 	// Test Ed25519 signatures
// 	if err := testEd25519(); err != nil {
// 		return fmt.Errorf("Ed25519 health check failed: %v", err)
// 	}
//
// 	return nil
// }
//
// func testECDH() error {
// 	curve := ecdh.X25519()
//
// 	privA, err := curve.GenerateKey(rand.Reader)
// 	if err != nil {
// 		return err
// 	}
//
// 	privB, err := curve.GenerateKey(rand.Reader)
// 	if err != nil {
// 		return err
// 	}
//
// 	shared1, err := privA.ECDH(privB.PublicKey())
// 	if err != nil {
// 		return err
// 	}
//
// 	shared2, err := privB.ECDH(privA.PublicKey())
// 	if err != nil {
// 		return err
// 	}
//
// 	if !bytes.Equal(shared1, shared2) {
// 		return errors.New("ECDH shared secrets don't match")
// 	}
//
// 	return nil
// }
