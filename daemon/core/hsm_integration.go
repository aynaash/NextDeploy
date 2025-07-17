package core
//
// import "crypto"
//
// type HSMSigner struct {
// 	ctx   HSMContext
// 	keyID string
// }
//
// func (s *HSMSigner) Public() crypto.PublicKey {
// 	pub, err := s.ctx.GetPublicKey(s.keyID)
// 	if err != nil {
// 		panic("HSM failure")
// 	}
// 	return pub
// }
//
// func (s *HSMSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
// 	return s.ctx.Sign(s.keyID, digest)
// }
//
// func NewHSMKeyManager(hsmConfig HSMConfig) (*KeyManager, error) {
// 	ctx, err := ConnectHSM(hsmConfig)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return &KeyManager{
// 		signer: &HSMSigner{ctx: ctx, keyID: "primary"},
// 		// ... other fields ...
// 	}, nil
// }
