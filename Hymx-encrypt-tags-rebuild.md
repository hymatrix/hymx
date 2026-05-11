我要对 encrypt_tags 进行重构：
- 现在的解密是在 node 层进行，下方到 vmm 层
- node 层不解密，只做传递，不要修改 tags 的 key ，直接传入 vmm 层
- 在 vmm 层进行解密，再传递给 vm
- 传递给 vm 的时候，保留原来的加密 tag，
- Meta 结构里新加 EncryptedParams map , 存放解密后的 tag 字段
- profix 变成 `Encrypted-`
- tag 的 key 不用在传递时去掉 `Encrypted-`前缀，保持一致
- checkpoint 只序列化 raw env + VM state + outbox
- restore 后 VMM 从 raw env 再解密
