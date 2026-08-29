package sealedreview

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/countersign"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

const (
	testDigestA            = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testDigestB            = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	wantEvidenceResultWire = `{"argv":["make","verify"],"command_id":"verify","digest":"sha256:c899e11a0c7383ce916fc594c817ddf9d1abc38cf33fb44dd8ebac0937b7c40b","exit_code":0,"output":"YnVpbGRlciBldmlkZW5jZQo=","output_digest":"sha256:21848dab15b49af4912153ffee26ccf7ab3d34df0cf7e0a3a5c640ee31948ec2","schema":"verdi.context-evidence-result/v1","verdict":"proven"}
`
	wantEvidenceBundleWire = `{"candidate":{"base_commit":"8ee9716b0184a862c4b9c292455bb204213c679c","base_tree":"7ab86bc996210e922210a37faeebc78acec98289","head_commit":"5078325fcede046f3e6fb58236cf5ad166145de8","head_tree":"1971e107db8b7cd51e76ccbb25b6bcb979e4a020"},"digest":"sha256:49fb5aeb91dc763e6fc9e2b0fa13a0daf1c6ffcaae7be90433f7afa6bf2d555c","rows":[{"command_id":"verify","result_bytes":"eyJhcmd2IjpbIm1ha2UiLCJ2ZXJpZnkiXSwiY29tbWFuZF9pZCI6InZlcmlmeSIsImRpZ2VzdCI6InNoYTI1NjpjODk5ZTExYTBjNzM4M2NlOTE2ZmM1OTRjODE3ZGRmOWQxYWJjMzhjZjMzZmI0NGRkOGViYWMwOTM3YjdjNDBiIiwiZXhpdF9jb2RlIjowLCJvdXRwdXQiOiJZblZwYkdSbGNpQmxkbWxrWlc1alpRbz0iLCJvdXRwdXRfZGlnZXN0Ijoic2hhMjU2OjIxODQ4ZGFiMTViNDlhZjQ5MTIxNTNmZmVlMjZjY2Y3YWIzZDM0ZGYwY2Y3ZTBhM2E1YzY0MGVlMzE5NDhlYzIiLCJzY2hlbWEiOiJ2ZXJkaS5jb250ZXh0LWV2aWRlbmNlLXJlc3VsdC92MSIsInZlcmRpY3QiOiJwcm92ZW4ifQo=","result_digest":"sha256:7bbf29ed7acb4ab788d51703c6eea6f8b93cd26c58e81fef29e68f99c4b776d8"}],"schema":"verdi.context-review-evidence-bundle/v1","scope":"builder"}
`
	wantDiffWire = `{"base_commit":"8ee9716b0184a862c4b9c292455bb204213c679c","base_tree":"7ab86bc996210e922210a37faeebc78acec98289","digest":"sha256:058c7b63bccd3835539f6874d827f9212c26ef84d9e98f67841ab9812b0c0f29","entries":[{"after_blob":"8d8873d3eaf2793cd1b74ffbf7204d478b7fc117","after_bytes":"cGF0aCBlZGdlCg==","after_mode":"100644","before_blob":"","before_bytes":"","before_mode":"","path":"IGxlYWRpbmcudHh0","state":"added"},{"after_blob":"8d8873d3eaf2793cd1b74ffbf7204d478b7fc117","after_bytes":"cGF0aCBlZGdlCg==","after_mode":"100644","before_blob":"","before_bytes":"","before_mode":"","path":"YVxi","state":"added"},{"after_blob":"294186e497a23bf3fbfde12aacc7f720f668fe9a","after_bytes":"YWZ0ZXIK","after_mode":"100644","before_blob":"90be1f3056c4f471f977a28497b8d4b392c55a02","before_bytes":"YmVmb3JlCg==","before_mode":"100644","path":"Y2hhbmdlZC50eHQ=","state":"modified"},{"after_blob":"1275430f1765c63e539cb0452565563bd6aef6a6","after_bytes":"c2FtZQo=","after_mode":"100644","before_blob":"","before_bytes":"","before_mode":"","path":"bmV3LnR4dA==","state":"added"},{"after_blob":"","after_bytes":"","after_mode":"","before_blob":"1275430f1765c63e539cb0452565563bd6aef6a6","before_bytes":"c2FtZQo=","before_mode":"100644","path":"b2xkLnR4dA==","state":"deleted"},{"after_blob":"8d8873d3eaf2793cd1b74ffbf7204d478b7fc117","after_bytes":"cGF0aCBlZGdlCg==","after_mode":"100644","before_blob":"","before_bytes":"","before_mode":"","path":"/y50eHQ=","state":"added"}],"head_commit":"5078325fcede046f3e6fb58236cf5ad166145de8","head_tree":"1971e107db8b7cd51e76ccbb25b6bcb979e4a020","schema":"verdi.context-review-diff/v1"}
`
	wantBindingR0Wire = `{"accepted_spec_digest":"sha256:be00c776eb65622e9f45a23ed5fc7b056317504193bd6f170295c1fea38d0ba9","builder_receipt_digest":"sha256:4436d01db6a6b54cde95e5e05e2e4a0866a21ad7ae4086f45d9e0f583552652e","digest":"sha256:6ef932c97d1895eabab7aec9032fcba0271ec7dd6859b56c00262574564762f8","head_commit":"5078325fcede046f3e6fb58236cf5ad166145de8","head_tree":"1971e107db8b7cd51e76ccbb25b6bcb979e4a020","item_projection":[{"content_digest":"sha256:be00c776eb65622e9f45a23ed5fc7b056317504193bd6f170295c1fea38d0ba9","kind":"accepted-spec"},{"content_digest":"sha256:2b3a873b403e3a88cefb86d21f1a1ddb096c3cafc7860467d6353faa07a9c542","kind":"builder-receipt"},{"content_digest":"sha256:541d0920a937c2dc6bf7f71f5a200e153c7f2a067c10052f750fdb1003b9db21","kind":"current-diff"},{"content_digest":"sha256:78a396bb20aed958dcad159335c9e8b9ad082967bfde6dce42d1e06ead269527","kind":"evidence-bundle"},{"content_digest":"sha256:50144c81c3dcc14fbb2a1209beeea006a4f0434716211666659b809603a244c9","kind":"review-policy"}],"packet_digest":"sha256:7725c3b2e52c7a539a781b9f539e6b425f01f87255aa760068d5436fe256b46e","review_policy_digest":"sha256:50144c81c3dcc14fbb2a1209beeea006a4f0434716211666659b809603a244c9","schema":"verdi.context-review-binding/v1"}
`
	wantBindingR2Wire = `{"accepted_spec_digest":"sha256:be00c776eb65622e9f45a23ed5fc7b056317504193bd6f170295c1fea38d0ba9","builder_receipt_digest":"sha256:4436d01db6a6b54cde95e5e05e2e4a0866a21ad7ae4086f45d9e0f583552652e","digest":"sha256:352fe2a4efb90d3d6490e5bf748d65758387ed493a5108ff8eed9a5dc40be9ce","head_commit":"5078325fcede046f3e6fb58236cf5ad166145de8","head_tree":"1971e107db8b7cd51e76ccbb25b6bcb979e4a020","item_projection":[{"content_digest":"sha256:be00c776eb65622e9f45a23ed5fc7b056317504193bd6f170295c1fea38d0ba9","kind":"accepted-spec"},{"content_digest":"sha256:4df038954a0ab8c007a18d24511c1c256a82cf7f4a5a98dcb5fb3969c16d6fc0","kind":"adjudication"},{"content_digest":"sha256:2b3a873b403e3a88cefb86d21f1a1ddb096c3cafc7860467d6353faa07a9c542","kind":"builder-receipt"},{"content_digest":"sha256:99dc7bbf8214cac45527552ab9fd21a2e718d2698d30dadaf5a8fa7a83db08d0","kind":"current-candidate-evidence"},{"content_digest":"sha256:541d0920a937c2dc6bf7f71f5a200e153c7f2a067c10052f750fdb1003b9db21","kind":"current-diff"},{"content_digest":"sha256:78a396bb20aed958dcad159335c9e8b9ad082967bfde6dce42d1e06ead269527","kind":"evidence-bundle"},{"content_digest":"sha256:50144c81c3dcc14fbb2a1209beeea006a4f0434716211666659b809603a244c9","kind":"review-policy"}],"packet_digest":"sha256:875b6a1fe725956dc06528155728ff461dde0df72a2d996081a051dfc7b279a4","review_policy_digest":"sha256:50144c81c3dcc14fbb2a1209beeea006a4f0434716211666659b809603a244c9","schema":"verdi.context-review-binding/v1"}
`
	wantAdjudicationWire = `{"digest":"sha256:1681ecda64c36bb4877fcfa408aea35a354b1fc75888760fe396af0ed7e03030","r0_receipt_digest":"sha256:3a4cdf2a286f603f54a29abc8bc219f1e2e2eedff56278eaf00ec441dcb15f1a","rows":[{"ack":{"epoch":"epoch-1","event_digest":"sha256:4a7d17b653783c8e5c955905e3de0b1ba108eb9470f25945d0d4035d65ab6b0c","flight":"flight-review","global_sequence":42,"kind":"adjudication","lane":"review","manifest_revision":0,"schema":"verdi.context-event-ack/v1","session":"session-r0","source_sequence":3},"event_bytes":"eyJhZGFwdGVyIjoiY29kZXgiLCJhZGFwdGVyX3ZlcnNpb24iOiIxLjIuMyIsImF0Y19ydW53YXkiOiIvcnVud2F5L3JldmlldyIsImNhbmRpZGF0ZV9jb21taXQiOiI1MDc4MzI1ZmNlZGUwNDZmM2U2ZmI1ODIzNmNmNWFkMTY2MTQ1ZGU4IiwiY2FuZGlkYXRlX3RyZWUiOiIxOTcxZTEwN2RiOGI3Y2Q1MWU3NmNjYmIyNWI2YmNiOTc5ZTRhMDIwIiwiZXBvY2giOiJlcG9jaC0xIiwiZXZlbnRfZGlnZXN0Ijoic2hhMjU2OjRhN2QxN2I2NTM3ODNjOGU1Yzk1NTkwNWUzZGUwYjFiYTEwOGViOTQ3MGYyNTk0NWQwZDQwMzVkNjVhYjZiMGMiLCJleGVjdXRpb25fd29ya3NwYWNlX2lkIjoid29ya3NwYWNlLXIwIiwiZmxpZ2h0IjoiZmxpZ2h0LXJldmlldyIsImtpbmQiOiJhZGp1ZGljYXRpb24iLCJsYW5lIjoicmV2aWV3IiwibWFuaWZlc3RfZGlnZXN0Ijoic2hhMjU2OmFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWEiLCJtYW5pZmVzdF9yZXZpc2lvbiI6MCwib2NjdXJyZWRfYXQiOiIyMDI2LTA4LTI5VDEyOjAwOjAwWiIsInBheWxvYWQiOnsiZGVjaXNpb24iOiJhY2NlcHQiLCJkZXRhaWwiOnsiZGlnZXN0Ijoic2hhMjU2OjZkZWVlYTY1ZDBhMGIyZGZjNTY2NmIwOWNmOTY3ODc5NWQ4YjZhYjczMmU5MmJkYWFhMjc3ZTkwNDZiZjNmYWEiLCJtZWRpYV90eXBlIjoiYXBwbGljYXRpb24vanNvbiIsIm1vZGUiOiJpbmxpbmUiLCJyZWRhY3RlZF9qc29uIjp7ImRlY2lzaW9uIjoiYWNjZXB0ZWQifSwicmVkYWN0aW9uX3Byb2ZpbGUiOiJ2ZXJkaS5yZWRhY3Rpb24vc3RhbmRhcmQtdjEifSwiZmluZGluZ19vcl9kZXZpYXRpb25faWQiOiJmaW5kaW5nLTEiLCJwcmluY2lwYWxfcmVzb2x1dGlvbiI6eyJjbGFpbSI6eyJzdWJqZWN0Ijoib3duZXJAZXhhbXBsZS5jb20iLCJ0cnVzdF9zb3VyY2UiOiJjaS1ydW5uZXIifSwicHJpbmNpcGFsX2lkIjoicHJpbmNpcGFsL2NpLXJ1bm5lci9iM2R1WlhKQVpYaGhiWEJzWlM1amIyMCIsInN0YXRlIjoiYXV0aGVudGljYXRlZCIsIndpdG5lc3NlcyI6W3siY29kZSI6InRydXN0LXN1YmplY3QtdmVyaWZpZWQiLCJldmlkZW5jZV9kaWdlc3QiOiJzaGEyNTY6YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYSIsInNvdXJjZV9pZCI6ImNpLXJ1bm5lciJ9XX0sInJlYXNvbl9kaWdlc3QiOiJzaGEyNTY6YmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYiIsInNjaGVtYSI6InZlcmRpLmNvbnRleHQtZXZlbnQtcGF5bG9hZC9hZGp1ZGljYXRpb24vdjEifSwicGF5bG9hZF9zY2hlbWEiOiJ2ZXJkaS5jb250ZXh0LWV2ZW50LXBheWxvYWQvYWRqdWRpY2F0aW9uL3YxIiwicHJpb3JfZXZlbnRfZGlnZXN0Ijoic2hhMjU2OmFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWEiLCJzY2hlbWEiOiJ2ZXJkaS5jb250ZXh0LWV2ZW50L3YxIiwic2Vzc2lvbiI6InNlc3Npb24tcjAiLCJzb3VyY2Vfc2VxdWVuY2UiOjN9Cg==","finding_or_deviation_id":"finding-1"}],"schema":"verdi.context-review-adjudication/v1"}
`
	wantPacketR0Wire = `{"builder_receipt_digest":"sha256:4436d01db6a6b54cde95e5e05e2e4a0866a21ad7ae4086f45d9e0f583552652e","candidate":{"base_commit":"8ee9716b0184a862c4b9c292455bb204213c679c","base_tree":"7ab86bc996210e922210a37faeebc78acec98289","head_commit":"5078325fcede046f3e6fb58236cf5ad166145de8","head_tree":"1971e107db8b7cd51e76ccbb25b6bcb979e4a020"},"digest":"sha256:7725c3b2e52c7a539a781b9f539e6b425f01f87255aa760068d5436fe256b46e","exclusions":["ambient-context","builder-conversation","global-memory","personal-memory","prior-reviewer-conversation"],"items":[{"content":"aWQ6IHNwZWMvcmV2aWV3LXRhcmdldApraW5kOiBzcGVjCmNsYXNzOiBmZWF0dXJlCnRpdGxlOiBSZXZpZXcgdGFyZ2V0CnN0YXR1czogZHJhZnQKb3duZXJzOiBbcGxhdGZvcm0tdGVhbV0KYWNjZXB0YW5jZV9jcml0ZXJpYToKICAtIHsgaWQ6IGFjLTEsIHRleHQ6IHJldmlld2VkLCBldmlkZW5jZTogW2JlaGF2aW9yYWxdIH0K","content_digest":"sha256:be00c776eb65622e9f45a23ed5fc7b056317504193bd6f170295c1fea38d0ba9","id":"spec/review-target","kind":"accepted-spec","media_type":"text/markdown; charset=utf-8"},{"content":"eyJiYXNlX2NvbW1pdCI6IjhlZTk3MTZiMDE4NGE4NjJjNGI5YzI5MjQ1NWJiMjA0MjEzYzY3OWMiLCJiYXNlX3RyZWUiOiI3YWI4NmJjOTk2MjEwZTkyMjIxMGEzN2ZhZWViYzc4YWNlYzk4Mjg5IiwiZGlnZXN0Ijoic2hhMjU2OjA1OGM3YjYzYmNjZDM4MzU1MzlmNjg3NGQ4MjdmOTIxMmMyNmVmODRkOWU5OGY2Nzg0MWFiOTgxMmIwYzBmMjkiLCJlbnRyaWVzIjpbeyJhZnRlcl9ibG9iIjoiOGQ4ODczZDNlYWYyNzkzY2QxYjc0ZmZiZjcyMDRkNDc4YjdmYzExNyIsImFmdGVyX2J5dGVzIjoiY0dGMGFDQmxaR2RsQ2c9PSIsImFmdGVyX21vZGUiOiIxMDA2NDQiLCJiZWZvcmVfYmxvYiI6IiIsImJlZm9yZV9ieXRlcyI6IiIsImJlZm9yZV9tb2RlIjoiIiwicGF0aCI6IklHeGxZV1JwYm1jdWRIaDAiLCJzdGF0ZSI6ImFkZGVkIn0seyJhZnRlcl9ibG9iIjoiOGQ4ODczZDNlYWYyNzkzY2QxYjc0ZmZiZjcyMDRkNDc4YjdmYzExNyIsImFmdGVyX2J5dGVzIjoiY0dGMGFDQmxaR2RsQ2c9PSIsImFmdGVyX21vZGUiOiIxMDA2NDQiLCJiZWZvcmVfYmxvYiI6IiIsImJlZm9yZV9ieXRlcyI6IiIsImJlZm9yZV9tb2RlIjoiIiwicGF0aCI6IllWeGkiLCJzdGF0ZSI6ImFkZGVkIn0seyJhZnRlcl9ibG9iIjoiMjk0MTg2ZTQ5N2EyM2JmM2ZiZmRlMTJhYWNjN2Y3MjBmNjY4ZmU5YSIsImFmdGVyX2J5dGVzIjoiWVdaMFpYSUsiLCJhZnRlcl9tb2RlIjoiMTAwNjQ0IiwiYmVmb3JlX2Jsb2IiOiI5MGJlMWYzMDU2YzRmNDcxZjk3N2EyODQ5N2I4ZDRiMzkyYzU1YTAyIiwiYmVmb3JlX2J5dGVzIjoiWW1WbWIzSmxDZz09IiwiYmVmb3JlX21vZGUiOiIxMDA2NDQiLCJwYXRoIjoiWTJoaGJtZGxaQzUwZUhRPSIsInN0YXRlIjoibW9kaWZpZWQifSx7ImFmdGVyX2Jsb2IiOiIxMjc1NDMwZjE3NjVjNjNlNTM5Y2IwNDUyNTY1NTYzYmQ2YWVmNmE2IiwiYWZ0ZXJfYnl0ZXMiOiJjMkZ0WlFvPSIsImFmdGVyX21vZGUiOiIxMDA2NDQiLCJiZWZvcmVfYmxvYiI6IiIsImJlZm9yZV9ieXRlcyI6IiIsImJlZm9yZV9tb2RlIjoiIiwicGF0aCI6ImJtVjNMblI0ZEE9PSIsInN0YXRlIjoiYWRkZWQifSx7ImFmdGVyX2Jsb2IiOiIiLCJhZnRlcl9ieXRlcyI6IiIsImFmdGVyX21vZGUiOiIiLCJiZWZvcmVfYmxvYiI6IjEyNzU0MzBmMTc2NWM2M2U1MzljYjA0NTI1NjU1NjNiZDZhZWY2YTYiLCJiZWZvcmVfYnl0ZXMiOiJjMkZ0WlFvPSIsImJlZm9yZV9tb2RlIjoiMTAwNjQ0IiwicGF0aCI6ImIyeGtMblI0ZEE9PSIsInN0YXRlIjoiZGVsZXRlZCJ9LHsiYWZ0ZXJfYmxvYiI6IjhkODg3M2QzZWFmMjc5M2NkMWI3NGZmYmY3MjA0ZDQ3OGI3ZmMxMTciLCJhZnRlcl9ieXRlcyI6ImNHRjBhQ0JsWkdkbENnPT0iLCJhZnRlcl9tb2RlIjoiMTAwNjQ0IiwiYmVmb3JlX2Jsb2IiOiIiLCJiZWZvcmVfYnl0ZXMiOiIiLCJiZWZvcmVfbW9kZSI6IiIsInBhdGgiOiIveTUwZUhRPSIsInN0YXRlIjoiYWRkZWQifV0sImhlYWRfY29tbWl0IjoiNTA3ODMyNWZjZWRlMDQ2ZjNlNmZiNTgyMzZjZjVhZDE2NjE0NWRlOCIsImhlYWRfdHJlZSI6IjE5NzFlMTA3ZGI4YjdjZDUxZTc2Y2NiYjI1YjZiY2I5NzllNGEwMjAiLCJzY2hlbWEiOiJ2ZXJkaS5jb250ZXh0LXJldmlldy1kaWZmL3YxIn0K","content_digest":"sha256:541d0920a937c2dc6bf7f71f5a200e153c7f2a067c10052f750fdb1003b9db21","id":"8ee9716b0184a862c4b9c292455bb204213c679c..5078325fcede046f3e6fb58236cf5ad166145de8","kind":"current-diff","media_type":"application/json"},{"content":"eyJjYW5kaWRhdGUiOnsiYmFzZV9jb21taXQiOiI4ZWU5NzE2YjAxODRhODYyYzRiOWMyOTI0NTViYjIwNDIxM2M2NzljIiwiYmFzZV90cmVlIjoiN2FiODZiYzk5NjIxMGU5MjIyMTBhMzdmYWVlYmM3OGFjZWM5ODI4OSIsImhlYWRfY29tbWl0IjoiNTA3ODMyNWZjZWRlMDQ2ZjNlNmZiNTgyMzZjZjVhZDE2NjE0NWRlOCIsImhlYWRfdHJlZSI6IjE5NzFlMTA3ZGI4YjdjZDUxZTc2Y2NiYjI1YjZiY2I5NzllNGEwMjAifSwiZGlnZXN0Ijoic2hhMjU2OjQ5ZmI1YWViOTFkYzc2M2U2ZmM5ZTJiMGZhMTNhMGRhZjFjNmZmY2FhZTdiZTkwNDMzZjdhZmE2YmYyZDU1NWMiLCJyb3dzIjpbeyJjb21tYW5kX2lkIjoidmVyaWZ5IiwicmVzdWx0X2J5dGVzIjoiZXlKaGNtZDJJanBiSW0xaGEyVWlMQ0oyWlhKcFpua2lYU3dpWTI5dGJXRnVaRjlwWkNJNkluWmxjbWxtZVNJc0ltUnBaMlZ6ZENJNkluTm9ZVEkxTmpwak9EazVaVEV4WVRCak56TTRNMk5sT1RFMlptTTFPVFJqT0RFM1pHUm1PV1F4WVdKak16aGpaak16Wm1JME5HUmtPR1ZpWVdNd09UTTNZamRqTkRCaUlpd2laWGhwZEY5amIyUmxJam93TENKdmRYUndkWFFpT2lKWmJsWndZa2RTYkdOcFFteGtiV3hyV2xjMWFscFJiejBpTENKdmRYUndkWFJmWkdsblpYTjBJam9pYzJoaE1qVTJPakl4T0RRNFpHRmlNVFZpTkRsaFpqUTVNVEl4TlRObVptVmxNalpqWTJZM1lXSXpaRE0wWkdZd1kyWTNaVEJoTTJFMVl6WTBNR1ZsTXpFNU5EaGxZeklpTENKelkyaGxiV0VpT2lKMlpYSmthUzVqYjI1MFpYaDBMV1YyYVdSbGJtTmxMWEpsYzNWc2RDOTJNU0lzSW5abGNtUnBZM1FpT2lKd2NtOTJaVzRpZlFvPSIsInJlc3VsdF9kaWdlc3QiOiJzaGEyNTY6N2JiZjI5ZWQ3YWNiNGFiNzg4ZDUxNzAzYzZlZWE2ZjhiOTNjZDI2YzU4ZTgxZmVmMjllNjhmOTljNGI3NzZkOCJ9XSwic2NoZW1hIjoidmVyZGkuY29udGV4dC1yZXZpZXctZXZpZGVuY2UtYnVuZGxlL3YxIiwic2NvcGUiOiJidWlsZGVyIn0K","content_digest":"sha256:78a396bb20aed958dcad159335c9e8b9ad082967bfde6dce42d1e06ead269527","id":"5078325fcede046f3e6fb58236cf5ad166145de8","kind":"evidence-bundle","media_type":"application/json"},{"content":"eyJhZGFwdGVyIjoiY29kZXgiLCJhZGFwdGVyX3ZlcnNpb24iOiIxLjIuMyIsImF0Y19ydW53YXkiOiIvcnVud2F5L3JldmlldyIsImF1dGhvcml0eSI6ImF1dGhvcml0YXRpdmUiLCJjbGVhbiI6dHJ1ZSwiZGlnZXN0Ijoic2hhMjU2OjQ0MzZkMDFkYjZhNmI1NGNkZTk1ZTVlMDVlMmU0YTA4NjZhMjFhZDdhZTQwODZmNDVkOWUwZjU4MzU1MjY1MmUiLCJkaXNwYXRjaF9kaWdlc3QiOiJzaGEyNTY6YmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYiIsImV2ZW50X2NoYWluX3Jvb3QiOiJzaGEyNTY6MWNlMjkzZjVkMWUyYWYzMDU3OWU2ZGFiMjQ2MGNjYjljZjY4M2FlYmEyMjFmMTEyYmJmNGI4MmIzZDg2MTE0MSIsImV2aWRlbmNlIjpbeyJhcmd2IjpbIm1ha2UiLCJ2ZXJpZnkiXSwiY29tbWFuZF9pZCI6InZlcmlmeSIsImV4aXRfY29kZSI6MCwib3V0cHV0X2RpZ2VzdCI6InNoYTI1NjoyMTg0OGRhYjE1YjQ5YWY0OTEyMTUzZmZlZTI2Y2NmN2FiM2QzNGRmMGNmN2UwYTNhNWM2NDBlZTMxOTQ4ZWMyIiwidmVyZGljdCI6InByb3ZlbiJ9XSwiZXhlY3V0aW9uX3dvcmtzcGFjZV9pZCI6IndvcmtzcGFjZS1yZXZpZXciLCJleGVjdXRpb25fd29ya3NwYWNlX3JlcXVlc3RfZGlnZXN0Ijoic2hhMjU2OmFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWEiLCJleHBhbnNpb25zIjpbXSwiaW5wdXRfY29tbWl0IjoiOGVlOTcxNmIwMTg0YTg2MmM0YjljMjkyNDU1YmIyMDQyMTNjNjc5YyIsImlucHV0X3RyZWUiOiI3YWI4NmJjOTk2MjEwZTkyMjIxMGEzN2ZhZWViYzc4YWNlYzk4Mjg5IiwibWFuaWZlc3RfZGlnZXN0Ijoic2hhMjU2OmFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWEiLCJvYmxpZ2F0aW9ucyI6W10sIm91dHB1dF9jb21taXQiOiI1MDc4MzI1ZmNlZGUwNDZmM2U2ZmI1ODIzNmNmNWFkMTY2MTQ1ZGU4Iiwib3V0cHV0X3RyZWUiOiIxOTcxZTEwN2RiOGI3Y2Q1MWU3NmNjYmIyNWI2YmNiOTc5ZTRhMDIwIiwicmV2aWV3X2lucHV0cyI6W10sInJldmlzaW9uX3NlZ21lbnRzIjpbeyJldmVudF9yb290Ijoic2hhMjU2OmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmIiLCJmaXJzdF9nbG9iYWxfc2VxdWVuY2UiOjEsIm1hbmlmZXN0X2RpZ2VzdCI6InNoYTI1NjphYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhIiwibWFuaWZlc3RfcmV2aXNpb24iOjAsInNjaGVtYSI6InZlcmRpLmNvbnRleHQtZXZlbnQtcmV2aXNpb24vdjEiLCJ0ZXJtaW5hbF9nbG9iYWxfc2VxdWVuY2UiOjIsInRlcm1pbmFsX2tpbmQiOiJleGVjdXRpb24tcmVzdWx0IiwidGVybWluYWxfc291cmNlX3NlcXVlbmNlIjoyfV0sInJvbGUiOiJidWlsZGVyIiwicnVubmVyX3ByaW5jaXBhbF9yZXNvbHV0aW9uIjp7ImNsYWltIjp7InN1YmplY3QiOiJyZXZpZXdlckBleGFtcGxlLmNvbSIsInRydXN0X3NvdXJjZSI6ImNpLXJ1bm5lciJ9LCJwcmluY2lwYWxfaWQiOiJwcmluY2lwYWwvY2ktcnVubmVyL2NtVjJhV1YzWlhKQVpYaGhiWEJzWlM1amIyMCIsInN0YXRlIjoiYXV0aGVudGljYXRlZCIsIndpdG5lc3NlcyI6W3siY29kZSI6InRydXN0LXN1YmplY3QtdmVyaWZpZWQiLCJldmlkZW5jZV9kaWdlc3QiOiJzaGEyNTY6YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYSIsInNvdXJjZV9pZCI6ImNpLXJ1bm5lciJ9XX0sInNjaGVtYSI6InZlcmRpLmNvbnRleHQtcmVjZWlwdC92MSIsInRlcm1pbmFsX2dsb2JhbF9zZXF1ZW5jZSI6MiwidGVybWluYWxfbWFuaWZlc3RfcmV2aXNpb24iOjAsInRlcm1pbmFsX3NvdXJjZV9zZXF1ZW5jZSI6Mn0K","content_digest":"sha256:2b3a873b403e3a88cefb86d21f1a1ddb096c3cafc7860467d6353faa07a9c542","id":"sha256:4436d01db6a6b54cde95e5e05e2e4a0866a21ad7ae4086f45d9e0f583552652e","kind":"builder-receipt","media_type":"application/json"},{"content":"LS0tCnNjaGVtYTogdmVyZGkucG9saWN5L3YxCmlkOiBwb2xpY3kvcmV2aWV3CmtpbmQ6IHBvbGljeQp0aXRsZTogIlJldmlldyBwb2xpY3kiCm93bmVyczogW3BsYXRmb3JtLXRlYW1dCnNjb3BlOiB7cGhhc2VzOiBbXSwgZW52aXJvbm1lbnRzOiBbXSwgcGF0aHM6IFtdLCByZWZzOiBbXX0KY2xhaW1zOiBbXQppbnN0cnVjdGlvbnM6CiAgLSAiUmV2aWV3IG9ubHkgdGhlIHN1cHBsaWVkIHBhY2tldC4iCnBheWxvYWRzOiB7fQp0ZW1wbGF0ZToge2lkZW50aXR5OiAiZW1iZWRkZWQ6cG9saWN5Lm1kIiwgZGlnZXN0OiAic2hhMjU2OjBlMWI4M2E4ZTQxZDVlY2ZlOWYxNGNiNDk3M2I3YTU4NGJmY2I0NzEyNDdmYTA2NGI1ZmUyNzNlNGQzMjI1NjEifQotLS0KUmV2aWV3IHBvbGljeSByYXRpb25hbGUuCg==","content_digest":"sha256:50144c81c3dcc14fbb2a1209beeea006a4f0434716211666659b809603a244c9","id":"policy/review","kind":"review-policy","media_type":"text/markdown; charset=utf-8"}],"reviewer":{"adapter":"codex","adapter_version":"1.2.3","lane":"review","model":"gpt-5.6","profile_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","profile_id":"reviewer-profile"},"round":"r0","schema":"verdi.context-review-packet/v1"}
`
)

func TestSealedReviewPacketContract_Behavioral(t *testing.T) {
	if got, want := PacketSchemaID, "verdi.context-review-packet/v1"; got != want {
		t.Fatalf("PacketSchemaID = %q, want %q", got, want)
	}
	if got, want := DiffSchemaID, "verdi.context-review-diff/v1"; got != want {
		t.Fatalf("DiffSchemaID = %q, want %q", got, want)
	}
	if got, want := EvidenceResultSchemaID, "verdi.context-evidence-result/v1"; got != want {
		t.Fatalf("EvidenceResultSchemaID = %q, want %q", got, want)
	}
	if got, want := EvidenceBundleSchemaID, "verdi.context-review-evidence-bundle/v1"; got != want {
		t.Fatalf("EvidenceBundleSchemaID = %q, want %q", got, want)
	}
	if got, want := AdjudicationSchemaID, "verdi.context-review-adjudication/v1"; got != want {
		t.Fatalf("AdjudicationSchemaID = %q, want %q", got, want)
	}
	if got, want := ReviewBindingSchemaID, "verdi.context-review-binding/v1"; got != want {
		t.Fatalf("ReviewBindingSchemaID = %q, want %q", got, want)
	}

	repository, candidate, acceptedSpec, reviewPolicy := reviewRepositoryFixture(t)
	builderReceipt, builderReceiptBytes := reviewBuilderReceiptFixture(t, candidate)
	builderEvidence := evidenceResultBytes(t, builderReceipt.Evidence[0], []byte("builder evidence\n"))
	currentEvidence := evidenceResultBytes(t, builderReceipt.Evidence[0], []byte("builder evidence\n"))
	compilerPort := &packetCompilerFake{rebuiltEvidence: [][]byte{currentEvidence}, manifest: reviewManifestFixture(t)}
	compiler, err := NewPacketCompiler(PacketCompilerPorts{Repository: repository, Compiler: compilerPort})
	if err != nil {
		t.Fatalf("NewPacketCompiler(valid ports) error = %v", err)
	}
	if _, err := NewPacketCompiler(PacketCompilerPorts{}); err == nil {
		t.Fatal("NewPacketCompiler(zero ports) error = nil")
	}

	reviewer := Reviewer{
		Lane: "review", Adapter: "codex", AdapterVersion: "1.2.3", Model: "gpt-5.6",
		ProfileID: "reviewer-profile", ProfileDigest: testDigestA,
	}
	baseRequest := PacketRequest{
		Round: RoundR0, Candidate: candidate, Reviewer: reviewer,
		AcceptedSpecPath: "spec.md", ReviewPolicyPath: "policy.md",
		BuilderReceiptBytes: builderReceiptBytes, BuilderEvidenceResultBytes: [][]byte{builderEvidence},
	}

	t.Run("context compilation rejects arbitrary bytes despite a matching sidecar", func(t *testing.T) {
		unboundPort := &packetCompilerFake{rebuiltEvidence: [][]byte{currentEvidence}, unboundCompilation: true}
		unboundCompiler, err := NewPacketCompiler(PacketCompilerPorts{Repository: repository, Compiler: unboundPort})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := unboundCompiler.Compile(context.Background(), baseRequest); err == nil {
			t.Fatal("Compile(arbitrary manifest and projection bytes) error = nil")
		}
	})

	r0, err := compiler.Compile(context.Background(), baseRequest)
	if err != nil {
		t.Fatalf("Compile(R0) error = %v", err)
	}
	wantR0Kinds := []ItemKind{"accepted-spec", "current-diff", "evidence-bundle", "builder-receipt", "review-policy"}
	wantExclusions := []string{"ambient-context", "builder-conversation", "global-memory", "personal-memory", "prior-reviewer-conversation"}
	assertPacketInventory(t, r0.Packet, wantR0Kinds, wantExclusions)
	if got, want := r0.Packet.Items[0].ID, "spec/review-target"; got != want {
		t.Fatalf("accepted-spec id = %q, want %q", got, want)
	}
	if got, want := r0.Packet.Items[0].MediaType, "text/markdown; charset=utf-8"; got != want {
		t.Fatalf("accepted-spec media type = %q, want %q", got, want)
	}
	if !bytes.Equal(r0.Packet.Items[0].Content, acceptedSpec) {
		t.Fatal("accepted-spec item did not preserve exact Git blob bytes")
	}
	if got, want := r0.Packet.Items[1].ID, candidate.BaseCommit+".."+candidate.HeadCommit; got != want {
		t.Fatalf("current-diff id = %q, want %q", got, want)
	}
	if got := r0.Packet.Items[2].ID; got != candidate.HeadCommit {
		t.Fatalf("evidence-bundle id = %q, want head commit", got)
	}
	if got := r0.Packet.Items[3].ID; got != builderReceipt.Digest {
		t.Fatalf("builder-receipt id = %q, want %q", got, builderReceipt.Digest)
	}
	if got, want := r0.Packet.Items[4].ID, "policy/review"; got != want {
		t.Fatalf("review-policy id = %q, want %q", got, want)
	}
	if !bytes.Equal(r0.Packet.Items[4].Content, reviewPolicy) {
		t.Fatal("review-policy item did not preserve exact Git blob bytes")
	}
	for _, index := range []int{1, 2, 3} {
		if got := r0.Packet.Items[index].MediaType; got != "application/json" {
			t.Fatalf("item[%d] media type = %q, want application/json", index, got)
		}
	}
	if got := rawDigestForTest(r0.Packet.Items[0].Content); got != r0.Packet.Items[0].ContentDigest {
		t.Fatalf("accepted-spec content digest = %q, want exact-byte digest %q", r0.Packet.Items[0].ContentDigest, got)
	}
	if decoded, err := DecodePacket(bytes.NewReader(r0.PacketBytes)); err != nil {
		t.Fatalf("DecodePacket(R0) error = %v", err)
	} else if !reflect.DeepEqual(decoded, r0.Packet) {
		t.Fatalf("DecodePacket(R0) changed packet\ngot:  %#v\nwant: %#v", decoded, r0.Packet)
	}
	if encoded, err := EncodePacket(r0.Packet); err != nil {
		t.Fatalf("EncodePacket(R0) error = %v", err)
	} else if !bytes.Equal(encoded, r0.PacketBytes) {
		t.Fatalf("EncodePacket(R0) changed bytes\ngot:  %s\nwant: %s", encoded, r0.PacketBytes)
	}

	wantDiff := []DiffEntry{
		{Path: []byte(" leading.txt"), State: DiffAdded, BeforeBytes: []byte{}, AfterMode: "100644", AfterBlob: repository.oidFor([]byte("path edge\n")), AfterBytes: []byte("path edge\n")},
		{Path: []byte(`a\b`), State: DiffAdded, BeforeBytes: []byte{}, AfterMode: "100644", AfterBlob: repository.oidFor([]byte("path edge\n")), AfterBytes: []byte("path edge\n")},
		{Path: []byte("changed.txt"), State: DiffModified, BeforeMode: "100644", BeforeBlob: repository.oidFor([]byte("before\n")), BeforeBytes: []byte("before\n"), AfterMode: "100644", AfterBlob: repository.oidFor([]byte("after\n")), AfterBytes: []byte("after\n")},
		{Path: []byte("new.txt"), State: DiffAdded, BeforeBytes: []byte{}, AfterMode: "100644", AfterBlob: repository.oidFor([]byte("same\n")), AfterBytes: []byte("same\n")},
		{Path: []byte("old.txt"), State: DiffDeleted, BeforeMode: "100644", BeforeBlob: repository.oidFor([]byte("same\n")), BeforeBytes: []byte("same\n"), AfterBytes: []byte{}},
		{Path: []byte{0xff, '.', 't', 'x', 't'}, State: DiffAdded, BeforeBytes: []byte{}, AfterMode: "100644", AfterBlob: repository.oidFor([]byte("path edge\n")), AfterBytes: []byte("path edge\n")},
	}
	if !reflect.DeepEqual(r0.Diff.Entries, wantDiff) {
		t.Fatalf("diff entries = %#v, want literal delete+add inventory %#v", r0.Diff.Entries, wantDiff)
	}
	if decoded, err := DecodeDiff(bytes.NewReader(r0.DiffBytes)); err != nil {
		t.Fatalf("DecodeDiff() error = %v", err)
	} else if !reflect.DeepEqual(decoded, r0.Diff) {
		t.Fatalf("DecodeDiff() changed diff\ngot:  %#v\nwant: %#v", decoded, r0.Diff)
	}
	if got := rawDigestForTest(r0.DiffBytes); got != r0.Packet.Items[1].ContentDigest {
		t.Fatalf("diff item digest = %q, want exact wrapper-byte digest %q", r0.Packet.Items[1].ContentDigest, got)
	}
	if got := r0.BuilderEvidence.Scope; got != EvidenceScopeBuilder {
		t.Fatalf("builder evidence scope = %q, want %q", got, EvidenceScopeBuilder)
	}
	if len(r0.BuilderEvidence.Rows) != 1 || r0.BuilderEvidence.Rows[0].CommandID != "verify" || !bytes.Equal(r0.BuilderEvidence.Rows[0].ResultBytes, builderEvidence) {
		t.Fatalf("builder evidence rows = %#v, want exact result wrapper", r0.BuilderEvidence.Rows)
	}
	if decoded, err := DecodeEvidenceBundle(bytes.NewReader(r0.Packet.Items[2].Content)); err != nil {
		t.Fatalf("DecodeEvidenceBundle(builder) error = %v", err)
	} else if decoded.Digest != r0.BuilderEvidence.Digest {
		t.Fatalf("decoded builder evidence digest = %q, want %q", decoded.Digest, r0.BuilderEvidence.Digest)
	}
	if len(compilerPort.compileRequests) != 1 {
		t.Fatalf("context compiler calls = %d, want 1", len(compilerPort.compileRequests))
	}
	assertContextBinding(t, compilerPort.compileRequests[0], r0)

	r0ReviewerReceipt, r0ReviewerReceiptBytes := reviewReceiptFixture(t, candidate, builderReceipt.Digest, packetProjection(r0.Packet))
	adjudicationBytes := reviewAdjudicationFixture(t, candidate, r0ReviewerReceipt.Digest)
	r2Request := baseRequest
	r2Request.Round = RoundR2
	r2Request.PriorReviewReceiptBytes = r0ReviewerReceiptBytes
	r2Request.AdjudicationBytes = adjudicationBytes
	r2, err := compiler.Compile(context.Background(), r2Request)
	if err != nil {
		t.Fatalf("Compile(R2) error = %v", err)
	}
	wantR2Kinds := []ItemKind{"accepted-spec", "current-diff", "evidence-bundle", "builder-receipt", "review-policy", "adjudication", "current-candidate-evidence"}
	assertPacketInventory(t, r2.Packet, wantR2Kinds, wantExclusions)
	if got := r2.Packet.Items[5].ID; got != r0ReviewerReceipt.Digest {
		t.Fatalf("adjudication id = %q, want actual R0 receipt %q", got, r0ReviewerReceipt.Digest)
	}
	if !bytes.Equal(r2.Packet.Items[5].Content, adjudicationBytes) {
		t.Fatal("R2 adjudication item changed exact acknowledged wrapper bytes")
	}
	if got := r2.Packet.Items[6].ID; got != candidate.HeadCommit {
		t.Fatalf("current-candidate-evidence id = %q, want head commit", got)
	}
	if r2.CurrentCandidateEvidence == nil || r2.CurrentCandidateEvidence.Scope != EvidenceScopeCurrentCandidate {
		t.Fatalf("current candidate evidence = %#v, want current-candidate bundle", r2.CurrentCandidateEvidence)
	}
	if r2.CurrentCandidateEvidence.Digest == r2.BuilderEvidence.Digest {
		t.Fatalf("R2 current evidence reused builder digest %q", r2.BuilderEvidence.Digest)
	}
	if compilerPort.rebuildCalls != 1 {
		t.Fatalf("fresh evidence rebuild calls = %d, want exactly 1 for R2", compilerPort.rebuildCalls)
	}
	if len(compilerPort.compileRequests) != 2 {
		t.Fatalf("context compiler calls = %d, want 2", len(compilerPort.compileRequests))
	}
	assertContextBinding(t, compilerPort.compileRequests[1], r2)
	bindingR0Bytes, err := EncodeContextBinding(compilerPort.compileRequests[0].Binding)
	if err != nil {
		t.Fatal(err)
	}
	bindingR2Bytes, err := EncodeContextBinding(compilerPort.compileRequests[1].Binding)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("sole producer freezes literal wrapper bytes and blank-digest preimages", func(t *testing.T) {
		for _, test := range []struct {
			name, want, digest string
			got                []byte
		}{
			{name: "evidence result", got: builderEvidence, want: wantEvidenceResultWire, digest: "sha256:c899e11a0c7383ce916fc594c817ddf9d1abc38cf33fb44dd8ebac0937b7c40b"},
			{name: "evidence bundle", got: r0.Packet.Items[2].Content, want: wantEvidenceBundleWire, digest: "sha256:49fb5aeb91dc763e6fc9e2b0fa13a0daf1c6ffcaae7be90433f7afa6bf2d555c"},
			{name: "diff", got: r0.DiffBytes, want: wantDiffWire, digest: "sha256:058c7b63bccd3835539f6874d827f9212c26ef84d9e98f67841ab9812b0c0f29"},
			{name: "adjudication", got: adjudicationBytes, want: wantAdjudicationWire, digest: "sha256:1681ecda64c36bb4877fcfa408aea35a354b1fc75888760fe396af0ed7e03030"},
			{name: "R0 binding", got: bindingR0Bytes, want: wantBindingR0Wire, digest: "sha256:6ef932c97d1895eabab7aec9032fcba0271ec7dd6859b56c00262574564762f8"},
			{name: "R2 binding", got: bindingR2Bytes, want: wantBindingR2Wire, digest: "sha256:352fe2a4efb90d3d6490e5bf748d65758387ed493a5108ff8eed9a5dc40be9ce"},
			{name: "packet", got: r0.PacketBytes, want: wantPacketR0Wire, digest: "sha256:7725c3b2e52c7a539a781b9f539e6b425f01f87255aa760068d5436fe256b46e"},
		} {
			t.Run(test.name, func(t *testing.T) {
				assertLiteralSelfDigest(t, test.got, test.want, test.digest)
			})
		}
	})

	t.Run("strict codecs reject byte and inventory mutations", func(t *testing.T) {
		packetBytes := r0.PacketBytes
		mutations := map[string][]byte{
			"unknown field":   bytes.Replace(packetBytes, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1),
			"duplicate field": bytes.Replace(packetBytes, []byte(`"schema":`), []byte(`"schema":"verdi.context-review-packet/v1","schema":`), 1),
			"null items":      bytes.Replace(packetBytes, []byte(`"items":[`), []byte(`"items":null,"discarded":[`), 1),
			"trailing data":   append(append([]byte(nil), packetBytes...), []byte("{}")...),
		}
		for name, raw := range mutations {
			t.Run(name, func(t *testing.T) {
				if _, err := DecodePacket(bytes.NewReader(raw)); err == nil {
					t.Fatal("DecodePacket(mutated) error = nil")
				}
			})
		}

		sha256OID := strings.Repeat("a", 64)
		if _, err := EncodeDiff(Diff{
			Schema: DiffSchemaID, BaseCommit: sha256OID, BaseTree: sha256OID,
			HeadCommit: sha256OID, HeadTree: sha256OID, Entries: []DiffEntry{},
		}); err == nil {
			t.Fatal("EncodeDiff(non-I-90 SHA-256 candidate) error = nil")
		}

		wrongOrder := r0.Packet
		wrongOrder.Digest = ""
		wrongOrder.Items = cloneItemsForTest(r0.Packet.Items)
		wrongOrder.Items[0], wrongOrder.Items[1] = wrongOrder.Items[1], wrongOrder.Items[0]
		if _, err := EncodePacket(wrongOrder); err == nil {
			t.Fatal("EncodePacket(reordered inventory) error = nil")
		}
		forbidden := r0.Packet
		forbidden.Digest = ""
		forbidden.Items = cloneItemsForTest(r0.Packet.Items)
		forbidden.Items[0].Kind = "builder-conversation"
		if _, err := EncodePacket(forbidden); err == nil {
			t.Fatal("EncodePacket(builder-conversation item) error = nil")
		}
		changedExclusions := r0.Packet
		changedExclusions.Digest = ""
		changedExclusions.Exclusions = append([]string(nil), r0.Packet.Exclusions...)
		changedExclusions.Exclusions[2] = "global-memory-allowed"
		if _, err := EncodePacket(changedExclusions); err == nil {
			t.Fatal("EncodePacket(changed memory exclusion) error = nil")
		}
		wrongContent := r0.Packet
		wrongContent.Digest = ""
		wrongContent.Items = cloneItemsForTest(r0.Packet.Items)
		wrongContent.Items[0].Content = append(wrongContent.Items[0].Content, 'x')
		if _, err := EncodePacket(wrongContent); err == nil {
			t.Fatal("EncodePacket(content digest mismatch) error = nil")
		}

		otherCandidate := candidate
		otherCandidate.HeadCommit = repository.oidFor([]byte("other head commit"))
		otherCandidate.HeadTree = repository.oidFor([]byte("other head tree"))
		otherReceipt, otherReceiptBytes := reviewBuilderReceiptFixture(t, otherCandidate)
		wrongReceiptCandidate := r0.Packet
		wrongReceiptCandidate.Digest = ""
		wrongReceiptCandidate.BuilderReceiptDigest = otherReceipt.Digest
		wrongReceiptCandidate.Items = cloneItemsForTest(r0.Packet.Items)
		wrongReceiptCandidate.Items[3].ID = otherReceipt.Digest
		wrongReceiptCandidate.Items[3].Content = otherReceiptBytes
		wrongReceiptCandidate.Items[3].ContentDigest = rawDigestForTest(otherReceiptBytes)
		if _, err := EncodePacket(wrongReceiptCandidate); err == nil {
			t.Fatal("EncodePacket(builder receipt from another candidate) error = nil")
		}
	})

	t.Run("non-null empty wrapper arrays survive strict round trips", func(t *testing.T) {
		emptyDiff := Diff{
			Schema: DiffSchemaID, BaseCommit: candidate.BaseCommit, BaseTree: candidate.BaseTree,
			HeadCommit: candidate.HeadCommit, HeadTree: candidate.HeadTree, Entries: []DiffEntry{},
		}
		diffBytes, err := EncodeDiff(emptyDiff)
		if err != nil {
			t.Fatal(err)
		}
		decodedDiff, err := DecodeDiff(bytes.NewReader(diffBytes))
		if err != nil {
			t.Fatal(err)
		}
		if decodedDiff.Entries == nil {
			t.Fatal("DecodeDiff(empty entries) returned nil array")
		}
		if again, err := EncodeDiff(decodedDiff); err != nil || !bytes.Equal(again, diffBytes) {
			t.Fatalf("empty diff round trip = %s/%v, want %s", again, err, diffBytes)
		}

		emptyBundle := EvidenceBundle{
			Schema: EvidenceBundleSchemaID, Scope: EvidenceScopeBuilder, Candidate: candidate, Rows: []EvidenceRow{},
		}
		bundleBytes, err := EncodeEvidenceBundle(emptyBundle)
		if err != nil {
			t.Fatal(err)
		}
		decodedBundle, err := DecodeEvidenceBundle(bytes.NewReader(bundleBytes))
		if err != nil {
			t.Fatal(err)
		}
		if decodedBundle.Rows == nil {
			t.Fatal("DecodeEvidenceBundle(empty rows) returned nil array")
		}
		if again, err := EncodeEvidenceBundle(decodedBundle); err != nil || !bytes.Equal(again, bundleBytes) {
			t.Fatalf("empty evidence bundle round trip = %s/%v, want %s", again, err, bundleBytes)
		}

		emptyAdjudication := Adjudication{Schema: AdjudicationSchemaID, R0ReceiptDigest: r0ReviewerReceipt.Digest, Rows: []AdjudicationRow{}}
		if _, err := EncodeAdjudication(emptyAdjudication); err == nil {
			t.Fatal("EncodeAdjudication(empty accepted rows) error = nil")
		}
	})

	t.Run("diff paths preserve the full Git byte domain", func(t *testing.T) {
		content := []byte("path edge\n")
		blob := repository.oidFor(content)
		diff := Diff{
			Schema: DiffSchemaID, BaseCommit: candidate.BaseCommit, BaseTree: candidate.BaseTree,
			HeadCommit: candidate.HeadCommit, HeadTree: candidate.HeadTree,
			Entries: []DiffEntry{
				{Path: []byte(" leading.txt"), State: DiffAdded, BeforeBytes: []byte{}, AfterMode: "100644", AfterBlob: blob, AfterBytes: content},
				{Path: []byte(`a\b`), State: DiffAdded, BeforeBytes: []byte{}, AfterMode: "100644", AfterBlob: blob, AfterBytes: content},
				{Path: []byte{0xff, '.', 't', 'x', 't'}, State: DiffAdded, BeforeBytes: []byte{}, AfterMode: "100644", AfterBlob: blob, AfterBytes: content},
			},
		}
		encoded, err := EncodeDiff(diff)
		if err != nil {
			t.Fatalf("EncodeDiff(raw paths) error = %v", err)
		}
		decoded, err := DecodeDiff(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("DecodeDiff(raw paths) error = %v", err)
		}
		for i := range diff.Entries {
			if !bytes.Equal(decoded.Entries[i].Path, diff.Entries[i].Path) {
				t.Fatalf("path[%d] = %x, want exact raw bytes %x", i, decoded.Entries[i].Path, diff.Entries[i].Path)
			}
		}
	})

	t.Run("stale source and evidence refuse before context compile", func(t *testing.T) {
		staleTree := baseRequest
		staleTree.Candidate.HeadTree = repository.oidFor([]byte("not the head tree"))
		before := len(compilerPort.compileRequests)
		if _, err := compiler.Compile(context.Background(), staleTree); err == nil {
			t.Fatal("Compile(stale head tree) error = nil")
		}
		if got := len(compilerPort.compileRequests); got != before {
			t.Fatalf("stale tree reached context compiler: calls %d -> %d", before, got)
		}

		staleEvidence := baseRequest
		other := builderReceipt.Evidence[0]
		other.ExitCode = 1
		other.Verdict = countersign.VerdictViolated
		staleEvidence.BuilderEvidenceResultBytes = [][]byte{evidenceResultBytes(t, other, []byte("builder evidence\n"))}
		if _, err := compiler.Compile(context.Background(), staleEvidence); err == nil {
			t.Fatal("Compile(stale evidence summary) error = nil")
		}

		staleCurrent := builderReceipt.Evidence[0]
		staleCurrent.ExitCode = 1
		staleCurrent.Verdict = countersign.VerdictViolated
		staleCurrent.OutputDigest = rawDigestForTest([]byte("stale current evidence\n"))
		staleCurrentBytes := evidenceResultBytes(t, staleCurrent, []byte("stale current evidence\n"))
		staleCurrentPort := &packetCompilerFake{rebuiltEvidence: [][]byte{staleCurrentBytes}, manifest: reviewManifestFixture(t)}
		staleCurrentCompiler, err := NewPacketCompiler(PacketCompilerPorts{Repository: repository, Compiler: staleCurrentPort})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := staleCurrentCompiler.Compile(context.Background(), r2Request); err == nil {
			t.Fatal("Compile(stale current-candidate evidence summary) error = nil")
		}
		if got := len(staleCurrentPort.compileRequests); got != 0 {
			t.Fatalf("stale current-candidate evidence reached context compiler: calls = %d", got)
		}
	})

	t.Run("repository object bodies must match commit tree and blob identities", func(t *testing.T) {
		for _, objectType := range []string{"commit", "tree", "blob"} {
			t.Run(objectType, func(t *testing.T) {
				corrupt := &reviewRepository{objects: repository.objects, corruptType: objectType}
				port := &packetCompilerFake{manifest: reviewManifestFixture(t)}
				compiler, err := NewPacketCompiler(PacketCompilerPorts{Repository: corrupt, Compiler: port})
				if err != nil {
					t.Fatal(err)
				}
				_, err = compiler.Compile(context.Background(), baseRequest)
				if err == nil {
					t.Fatalf("Compile(corrupt %s body) error = nil", objectType)
				}
				if !strings.Contains(err.Error(), "content identity mismatch") {
					t.Fatalf("Compile(corrupt %s body) error = %q, want object identity rejection", objectType, err)
				}
			})
		}
	})

	t.Run("adjudication requires an exact durable event acknowledgment", func(t *testing.T) {
		adjudication, err := DecodeAdjudication(bytes.NewReader(adjudicationBytes))
		if err != nil {
			t.Fatal(err)
		}
		adjudication.Digest = ""
		adjudication.Rows[0].Ack.EventDigest = testDigestB
		if _, err := EncodeAdjudication(adjudication); err == nil {
			t.Fatal("EncodeAdjudication(mismatched ack) error = nil")
		}
	})

	t.Run("R2 rejects a durably acknowledged rejected adjudication", func(t *testing.T) {
		rejected := rewriteAdjudicationForTest(t, adjudicationBytes, func(_ *contextevent.Event, payload *contextevent.AdjudicationPayload) {
			payload.Decision = "reject"
		})
		request := r2Request
		request.AdjudicationBytes = rejected
		if _, err := compiler.Compile(context.Background(), request); err == nil {
			t.Fatal("Compile(R2 with rejected adjudication) error = nil")
		}
	})

	t.Run("R2 requires exact accepted R0 lineage", func(t *testing.T) {
		nonR0Receipt, nonR0ReceiptBytes := reviewReceiptFixture(t, candidate, builderReceipt.Digest, packetProjection(r2.Packet))
		nonR0Request := r2Request
		nonR0Request.PriorReviewReceiptBytes = nonR0ReceiptBytes
		nonR0Request.AdjudicationBytes = reviewAdjudicationFixture(t, candidate, nonR0Receipt.Digest)
		if _, err := compiler.Compile(context.Background(), nonR0Request); err == nil {
			t.Fatal("Compile(R2 with seven-kind prior receipt) error = nil")
		}

		wrongInputReceipt := r0ReviewerReceipt
		wrongInputReceipt.Digest = ""
		wrongInputReceipt.ReviewInputs = append([]contextreceipt.ReviewInput(nil), r0ReviewerReceipt.ReviewInputs...)
		wrongInputReceipt.ReviewInputs[0].ContentDigest = testDigestB
		wrongInputReceiptBytes, err := contextreceipt.EncodeReceipt(wrongInputReceipt)
		if err != nil {
			t.Fatal(err)
		}
		wrongInputReceipt, err = contextreceipt.DecodeReceipt(bytes.NewReader(wrongInputReceiptBytes))
		if err != nil {
			t.Fatal(err)
		}
		wrongInputRequest := r2Request
		wrongInputRequest.PriorReviewReceiptBytes = wrongInputReceiptBytes
		wrongInputRequest.AdjudicationBytes = reviewAdjudicationFixture(t, candidate, wrongInputReceipt.Digest)
		if _, err := compiler.Compile(context.Background(), wrongInputRequest); err == nil {
			t.Fatal("Compile(R2 with wrong R0 item digest) error = nil")
		}

		for _, mutation := range []struct {
			name   string
			mutate func(*contextevent.Event, *contextevent.AdjudicationPayload)
		}{
			{name: "unauthenticated principal", mutate: func(_ *contextevent.Event, payload *contextevent.AdjudicationPayload) {
				payload.PrincipalResolution.State = gp.ResolutionUnproven
				payload.PrincipalResolution.PrincipalID = ""
			}},
			{name: "wrong candidate", mutate: func(event *contextevent.Event, _ *contextevent.AdjudicationPayload) {
				event.CandidateCommit = repository.oidFor([]byte("other adjudication commit"))
				event.CandidateTree = repository.oidFor([]byte("other adjudication tree"))
			}},
		} {
			t.Run(mutation.name, func(t *testing.T) {
				request := r2Request
				request.AdjudicationBytes = rewriteAdjudicationForTest(t, adjudicationBytes, mutation.mutate)
				if _, err := compiler.Compile(context.Background(), request); err == nil {
					t.Fatal("Compile(R2 with mutated adjudication lineage) error = nil")
				}
			})
		}

		wrongReceipt := rewriteAdjudicationReceiptForTest(t, adjudicationBytes, testDigestB)
		request := r2Request
		request.AdjudicationBytes = wrongReceipt
		if _, err := compiler.Compile(context.Background(), request); err == nil {
			t.Fatal("Compile(R2 with wrong adjudication receipt digest) error = nil")
		}
	})

	t.Run("compiler sidecar is convenience only", func(t *testing.T) {
		brokenPort := &packetCompilerFake{manifest: reviewManifestFixture(t), mutateBinding: func(binding *ContextBinding) { binding.HeadTree = repository.oidFor([]byte("other tree")) }}
		broken, err := NewPacketCompiler(PacketCompilerPorts{Repository: repository, Compiler: brokenPort})
		if err != nil {
			t.Fatal(err)
		}
		result, err := broken.Compile(context.Background(), baseRequest)
		if err != nil {
			t.Fatalf("Compile(valid bytes with mismatched convenience sidecar) error = %v", err)
		}
		if result.Compilation.Binding.HeadTree != candidate.HeadTree {
			t.Fatalf("result sidecar head tree = %q, want recomputed %q", result.Compilation.Binding.HeadTree, candidate.HeadTree)
		}
	})

	t.Run("context compilation cross-matches exact manifest and projection bytes", func(t *testing.T) {
		mutations := []struct {
			name   string
			mutate func(*ContextCompileResult)
		}{
			{name: "malformed manifest", mutate: func(result *ContextCompileResult) {
				result.ManifestBytes = []byte("{}\n")
				result.ManifestDigest = rawDigestForTest(result.ManifestBytes)
			}},
			{name: "manifest result digest", mutate: func(result *ContextCompileResult) {
				result.ManifestDigest = testDigestB
			}},
			{name: "repository head", mutate: func(result *ContextCompileResult) {
				rewriteManifestForTest(t, result, func(manifest *contextcompile.Manifest) {
					manifest.Repository.Head.Value = repository.oidFor([]byte("other manifest head"))
				})
			}},
			{name: "accepted spec", mutate: func(result *ContextCompileResult) {
				rewriteManifestForTest(t, result, func(manifest *contextcompile.Manifest) {
					manifest.AcceptedSpec.ContentDigest = testDigestB
				})
			}},
			{name: "adapter", mutate: func(result *ContextCompileResult) {
				rewriteManifestForTest(t, result, func(manifest *contextcompile.Manifest) {
					manifest.Adapter.Version = "9.9.9"
					for i := range manifest.Opaque {
						manifest.Opaque[i].Adapter = manifest.Adapter
						manifest.Opaque[i].ID = "opaque:harness-vendor-base/" + manifest.Adapter.ID + "/" + manifest.Adapter.Version
					}
				})
			}},
			{name: "profile", mutate: func(result *ContextCompileResult) {
				rewriteManifestForTest(t, result, func(manifest *contextcompile.Manifest) {
					manifest.GovernanceProfile.Digest = testDigestB
					manifest.Policy.ProfileDigest = testDigestB
				})
			}},
			{name: "projection digest row", mutate: func(result *ContextCompileResult) {
				rewriteManifestForTest(t, result, func(manifest *contextcompile.Manifest) {
					manifest.ProjectionFiles[0].Digest = testDigestB
				})
			}},
			{name: "coherent but packet-unbound projection", mutate: func(result *ContextCompileResult) {
				result.InstructionProjectionBytes = append(result.InstructionProjectionBytes, []byte("unbound\n")...)
				result.InstructionProjectionDigest = rawDigestForTest(result.InstructionProjectionBytes)
				rewriteManifestForTest(t, result, func(manifest *contextcompile.Manifest) {
					manifest.ProjectionFiles[0].Digest = result.InstructionProjectionDigest
				})
			}},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				port := &packetCompilerFake{manifest: reviewManifestFixture(t), mutateCompilation: mutation.mutate}
				compiler, err := NewPacketCompiler(PacketCompilerPorts{Repository: repository, Compiler: port})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := compiler.Compile(context.Background(), baseRequest); err == nil {
					t.Fatal("Compile(mutated context compilation) error = nil")
				}
			})
		}
	})

	t.Run("review proof emits consumer-valid unproven freshness without acknowledged launch facts", func(t *testing.T) {
		reviewerReceipt, _ := reviewReceiptFixture(t, candidate, builderReceipt.Digest, packetProjection(r0.Packet))
		projection, err := VerifyReviewProof(r0.PacketBytes, reviewerReceipt, candidate)
		if err != nil {
			t.Fatalf("VerifyReviewProof(exact R0) error = %v", err)
		}
		for name, operand := range map[string]contextreceipt.ReviewOperandProjection{
			"packet": projection.Packet, "link": projection.Link,
		} {
			if operand.State != contextreceipt.StateProven || operand.ExpectedDigest != operand.ObservedDigest {
				t.Fatalf("%s projection = %#v, want exact proven equality", name, operand)
			}
		}
		const wantFreshness = "sha256:d6894bfe4df20415d8bb1bf56a68c02266d3ae7e56b09aa8a1b5cf014ed1e802"
		if projection.Freshness.State != contextreceipt.StateUnproven || projection.Freshness.ExpectedDigest != wantFreshness || projection.Freshness.ObservedDigest != "" {
			t.Fatalf("freshness projection = %#v, want consumer-valid expected candidate/link digest %q with absent observation", projection.Freshness, wantFreshness)
		}

		staleCandidate := candidate
		staleCandidate.HeadCommit = repository.oidFor([]byte("other commit"))
		projection, err = VerifyReviewProof(r0.PacketBytes, reviewerReceipt, staleCandidate)
		if err != nil {
			t.Fatalf("VerifyReviewProof(stale candidate) operational error = %v", err)
		}
		const wantStaleFreshness = "sha256:cccaf330881e749ffbc8fd2601fcd26f80309bd47c3c5d34b6994d284a523bf2"
		if projection.Freshness.State != contextreceipt.StateUnproven || projection.Freshness.ExpectedDigest != wantStaleFreshness || projection.Freshness.ObservedDigest != "" {
			t.Fatalf("stale freshness projection = %#v, want candidate-sensitive expected digest %q with absent observation", projection.Freshness, wantStaleFreshness)
		}

		selfAuthenticated := r0.Packet
		selfAuthenticated.Digest = ""
		selfAuthenticated.Reviewer.Model = "self-asserted-model"
		selfAuthenticatedBytes, err := EncodePacket(selfAuthenticated)
		if err != nil {
			t.Fatal(err)
		}
		projection, err = VerifyReviewProof(selfAuthenticatedBytes, reviewerReceipt, candidate)
		if err != nil {
			t.Fatalf("VerifyReviewProof(self-authenticated reviewer) operational error = %v", err)
		}
		if projection.Freshness.State != contextreceipt.StateUnproven || projection.Freshness.ExpectedDigest != wantFreshness || projection.Freshness.ObservedDigest != "" {
			t.Fatalf("self-authenticated reviewer freshness = %#v, want packet-reviewer-independent expected digest %q with absent observation", projection.Freshness, wantFreshness)
		}

		wrongLink := reviewerReceipt
		wrongLink.ReviewOf = []string{testDigestB}
		wrongLink.Digest = ""
		projection, err = VerifyReviewProof(r0.PacketBytes, wrongLink, candidate)
		if err != nil {
			t.Fatalf("VerifyReviewProof(wrong link) operational error = %v", err)
		}
		if projection.Link.State != contextreceipt.StateViolated || projection.Link.ExpectedDigest == projection.Link.ObservedDigest {
			t.Fatalf("wrong link projection = %#v, want violated distinct digests", projection.Link)
		}
		const wantWrongLinkFreshness = "sha256:721cb2fb7a123d2e2b4b94ba3e99caaf7e591517c5dc6d6e404c88f23b635ee1"
		if projection.Freshness.State != contextreceipt.StateUnproven || projection.Freshness.ExpectedDigest != wantWrongLinkFreshness || projection.Freshness.ObservedDigest != "" {
			t.Fatalf("wrong-link freshness projection = %#v, want ReviewOf-sensitive expected digest %q with absent observation", projection.Freshness, wantWrongLinkFreshness)
		}

		wrongPacket := reviewerReceipt
		wrongPacket.ReviewInputs = []contextreceipt.ReviewInput{}
		wrongPacket.Digest = ""
		projection, err = VerifyReviewProof(r0.PacketBytes, wrongPacket, candidate)
		if err != nil {
			t.Fatalf("VerifyReviewProof(wrong packet projection) operational error = %v", err)
		}
		if projection.Packet.State != contextreceipt.StateViolated || projection.Packet.ExpectedDigest == projection.Packet.ObservedDigest {
			t.Fatalf("wrong packet projection = %#v, want violated distinct digests", projection.Packet)
		}
		if projection.Freshness.State != contextreceipt.StateUnproven || projection.Freshness.ExpectedDigest != wantFreshness || projection.Freshness.ObservedDigest != "" {
			t.Fatalf("wrong-packet freshness projection = %#v, want review-input-independent expected digest %q with absent observation", projection.Freshness, wantFreshness)
		}

		if _, err := VerifyReviewProof([]byte("{}\n"), reviewerReceipt, candidate); err == nil {
			t.Fatal("VerifyReviewProof(malformed packet) error = nil")
		}
	})

	var nilContext context.Context
	if _, err := compiler.Compile(nilContext, baseRequest); err == nil {
		t.Fatal("Compile(nil context) error = nil")
	}
}

func assertPacketInventory(t *testing.T, packet Packet, wantKinds []ItemKind, wantExclusions []string) {
	t.Helper()
	gotKinds := make([]ItemKind, len(packet.Items))
	for i, item := range packet.Items {
		gotKinds[i] = item.Kind
	}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("item kinds = %#v, want exact literal order %#v", gotKinds, wantKinds)
	}
	if !reflect.DeepEqual(packet.Exclusions, wantExclusions) {
		t.Fatalf("exclusions = %#v, want exact literal %#v", packet.Exclusions, wantExclusions)
	}
}

func assertLiteralSelfDigest(t *testing.T, got []byte, want, digest string) {
	t.Helper()
	wantBytes := []byte(want)
	if !bytes.Equal(got, wantBytes) {
		t.Fatalf("canonical bytes changed\ngot:  %q\nwant: %q", got, wantBytes)
	}
	digestField := []byte(`"digest":"` + digest + `"`)
	if count := bytes.Count(wantBytes, digestField); count != 1 {
		t.Fatalf("literal digest field count = %d, want exactly 1", count)
	}
	blankPreimage := bytes.Replace(wantBytes, digestField, []byte(`"digest":""`), 1)
	if gotDigest := rawDigestForTest(blankPreimage); gotDigest != digest {
		t.Fatalf("literal blank-digest preimage digest = %q, want %q", gotDigest, digest)
	}
}

func assertContextBinding(t *testing.T, request ContextCompileRequest, result PacketResult) {
	t.Helper()
	if !bytes.Equal(request.PacketBytes, result.PacketBytes) {
		t.Fatal("context compiler did not receive the exact packet bytes")
	}
	if !reflect.DeepEqual(request.Binding, result.Compilation.Binding) {
		t.Fatalf("context binding = %#v, compiled echo %#v", request.Binding, result.Compilation.Binding)
	}
	want := ContextBinding{
		Schema:               ReviewBindingSchemaID,
		PacketDigest:         result.Packet.Digest,
		AcceptedSpecDigest:   result.Packet.Items[0].ContentDigest,
		ReviewPolicyDigest:   result.Packet.Items[4].ContentDigest,
		BuilderReceiptDigest: result.Packet.BuilderReceiptDigest,
		HeadCommit:           result.Packet.Candidate.HeadCommit, HeadTree: result.Packet.Candidate.HeadTree,
		ItemProjection: packetProjection(result.Packet),
		Digest:         request.Binding.Digest,
	}
	if !reflect.DeepEqual(request.Binding, want) {
		t.Fatalf("context binding = %#v, want exact packet-derived %#v", request.Binding, want)
	}
}

func packetProjection(packet Packet) []contextreceipt.ReviewInput {
	rows := make([]contextreceipt.ReviewInput, len(packet.Items))
	for i, item := range packet.Items {
		rows[i] = contextreceipt.ReviewInput{Kind: string(item.Kind), ContentDigest: item.ContentDigest}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		return rows[i].ContentDigest < rows[j].ContentDigest
	})
	return rows
}

func cloneItemsForTest(items []Item) []Item {
	cloned := append([]Item(nil), items...)
	for i := range cloned {
		cloned[i].Content = append([]byte(nil), cloned[i].Content...)
	}
	return cloned
}

type packetCompilerFake struct {
	rebuiltEvidence    [][]byte
	rebuildCalls       int
	compileRequests    []ContextCompileRequest
	manifest           contextcompile.Manifest
	unboundCompilation bool
	mutateBinding      func(*ContextBinding)
	mutateCompilation  func(*ContextCompileResult)
}

func (f *packetCompilerFake) RebuildEvidence(_ context.Context, _ EvidenceRebuildRequest) ([][]byte, error) {
	f.rebuildCalls++
	return cloneByteRowsForTest(f.rebuiltEvidence), nil
}

func (f *packetCompilerFake) Compile(_ context.Context, request ContextCompileRequest) (ContextCompileResult, error) {
	request.PacketBytes = append([]byte(nil), request.PacketBytes...)
	request.Binding.ItemProjection = append([]contextreceipt.ReviewInput(nil), request.Binding.ItemProjection...)
	f.compileRequests = append(f.compileRequests, request)
	binding := request.Binding
	binding.ItemProjection = append([]contextreceipt.ReviewInput(nil), binding.ItemProjection...)
	if f.mutateBinding != nil {
		f.mutateBinding(&binding)
	}
	if f.unboundCompilation {
		manifest := []byte("{\"manifest\":\"packet-bound\"}\n")
		projection := []byte("Review only the exact packet data.\n")
		return ContextCompileResult{
			ManifestBytes: manifest, ManifestDigest: rawDigestForTest(manifest),
			InstructionProjectionBytes: projection, InstructionProjectionDigest: rawDigestForTest(projection),
			Binding: binding,
		}, nil
	}
	packet, err := DecodePacket(bytes.NewReader(request.PacketBytes))
	if err != nil {
		return ContextCompileResult{}, err
	}
	bindingBytes, err := EncodeContextBinding(request.Binding)
	if err != nil {
		return ContextCompileResult{}, err
	}
	projection := []byte("<!-- verdi:review-binding " + base64.StdEncoding.EncodeToString(bindingBytes) + " -->\n")
	projection = append(projection, packet.Items[4].Content...)

	manifest := f.manifest
	manifest.Phase = contextcompile.PhaseReview
	manifest.Adapter.ID = string(request.Reviewer.Adapter)
	manifest.Adapter.Version = request.Reviewer.AdapterVersion
	manifest.AcceptedSpec.Ref = packet.Items[0].ID
	manifest.AcceptedSpec.Blob = testGitObjectOID("blob", packet.Items[0].Content)
	manifest.AcceptedSpec.Commit = request.Candidate.HeadCommit
	manifest.AcceptedSpec.ContentDigest = packet.Items[0].ContentDigest
	manifest.Repository.Head.Known = true
	manifest.Repository.Head.Value = request.Candidate.HeadCommit
	manifest.Policy.ProfileID = request.Reviewer.ProfileID
	manifest.Policy.ProfileDigest = request.Reviewer.ProfileDigest
	manifest.GovernanceProfile.ID = request.Reviewer.ProfileID
	manifest.GovernanceProfile.Digest = request.Reviewer.ProfileDigest
	manifest.Scope.Phases = []string{"review"}
	manifest.ProjectionFiles = []contextcompile.ProjectionFileRef{{Path: "AGENTS.md", Digest: rawDigestForTest(projection)}}
	manifest.RequiredInputs = reviewRequiredInputs(packet)
	manifest.Opaque = append([]contextcompile.OpaqueEntry(nil), manifest.Opaque...)
	for i := range manifest.Opaque {
		manifest.Opaque[i].Adapter = manifest.Adapter
	}
	manifestBytes, err := contextcompile.EncodeManifest(manifest)
	if err != nil {
		return ContextCompileResult{}, err
	}
	manifest, err = contextcompile.DecodeManifest(manifestBytes)
	if err != nil {
		return ContextCompileResult{}, err
	}
	result := ContextCompileResult{
		ManifestBytes: manifestBytes, ManifestDigest: manifest.Digest,
		InstructionProjectionBytes: projection, InstructionProjectionDigest: rawDigestForTest(projection),
		Binding: binding,
	}
	if f.mutateCompilation != nil {
		f.mutateCompilation(&result)
	}
	return result, nil
}

func reviewManifestFixture(t *testing.T) contextcompile.Manifest {
	t.Helper()
	raw, err := os.ReadFile("../contextcompile/testdata/manifest-build.json")
	if err != nil {
		t.Fatalf("read context manifest fixture: %v", err)
	}
	manifest, err := contextcompile.DecodeManifest(raw)
	if err != nil {
		t.Fatalf("decode context manifest fixture: %v", err)
	}
	return manifest
}

func rewriteManifestForTest(t *testing.T, result *ContextCompileResult, mutate func(*contextcompile.Manifest)) {
	t.Helper()
	manifest, err := contextcompile.DecodeManifest(result.ManifestBytes)
	if err != nil {
		t.Fatalf("decode manifest for mutation: %v", err)
	}
	mutate(&manifest)
	result.ManifestBytes, err = contextcompile.EncodeManifest(manifest)
	if err != nil {
		t.Fatalf("encode manifest mutation: %v", err)
	}
	manifest, err = contextcompile.DecodeManifest(result.ManifestBytes)
	if err != nil {
		t.Fatalf("decode mutated manifest: %v", err)
	}
	result.ManifestDigest = manifest.Digest
}

func reviewRequiredInputs(packet Packet) []contextcompile.RequiredInput {
	inputs := []struct {
		kind  string
		index int
	}{
		{contextcompile.RequiredInputAcceptedSpec, 0},
		{contextcompile.RequiredInputBuilderReceipt, 3},
		{contextcompile.RequiredInputEvidenceBundle, 2},
		{contextcompile.RequiredInputResultDiff, 1},
		{contextcompile.RequiredInputReviewPolicy, 4},
	}
	rows := make([]contextcompile.RequiredInput, len(inputs))
	for i, input := range inputs {
		digest := packet.Items[input.index].ContentDigest
		rows[i] = contextcompile.RequiredInput{Kind: input.kind, Resolution: contextcompile.ResolutionProven, Digest: &digest, Witnesses: []string{}}
	}
	return rows
}

func cloneByteRowsForTest(rows [][]byte) [][]byte {
	cloned := make([][]byte, len(rows))
	for i := range rows {
		cloned[i] = append([]byte(nil), rows[i]...)
	}
	return cloned
}

type reviewRepository struct {
	objects     map[string]RepositoryObject
	corruptType string
}

func (r *reviewRepository) ReadObject(_ context.Context, oid string) (RepositoryObject, error) {
	object, ok := r.objects[oid]
	if !ok {
		return RepositoryObject{}, fmt.Errorf("object %s not found", oid)
	}
	object.Content = append([]byte(nil), object.Content...)
	if object.Type == r.corruptType {
		object.Content = append(object.Content, '!')
	}
	return object, nil
}

func (r *reviewRepository) oidFor(content []byte) string {
	return testGitObjectOID("blob", content)
}

func reviewRepositoryFixture(t *testing.T) (*reviewRepository, contextreceipt.Candidate, []byte, []byte) {
	t.Helper()
	acceptedSpec := []byte("id: spec/review-target\nkind: spec\nclass: feature\ntitle: Review target\nstatus: draft\nowners: [platform-team]\nacceptance_criteria:\n  - { id: ac-1, text: reviewed, evidence: [behavioral] }\n")
	reviewPolicy := []byte(`---
schema: verdi.policy/v1
id: policy/review
kind: policy
title: "Review policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions:
  - "Review only the supplied packet."
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Review policy rationale.
`)
	objects := make(map[string]RepositoryObject)
	add := func(kind string, content []byte) string {
		oid := testGitObjectOID(kind, content)
		objects[oid] = RepositoryObject{Type: kind, Content: append([]byte(nil), content...)}
		return oid
	}
	beforeBlob := add("blob", []byte("before\n"))
	afterBlob := add("blob", []byte("after\n"))
	sameBlob := add("blob", []byte("same\n"))
	pathEdgeBlob := add("blob", []byte("path edge\n"))
	specBlob := add("blob", acceptedSpec)
	policyBlob := add("blob", reviewPolicy)
	baseTreeBody := gitTreeBody(t, []gitTreeFixtureEntry{
		{"100644", "changed.txt", beforeBlob}, {"100644", "old.txt", sameBlob},
		{"100644", "policy.md", policyBlob}, {"100644", "spec.md", specBlob},
	})
	headTreeBody := gitTreeBody(t, []gitTreeFixtureEntry{
		{"100644", " leading.txt", pathEdgeBlob}, {"100644", `a\b`, pathEdgeBlob},
		{"100644", "changed.txt", afterBlob}, {"100644", "new.txt", sameBlob},
		{"100644", "policy.md", policyBlob}, {"100644", "spec.md", specBlob},
		{"100644", string([]byte{0xff, '.', 't', 'x', 't'}), pathEdgeBlob},
	})
	baseTree := add("tree", baseTreeBody)
	headTree := add("tree", headTreeBody)
	baseCommit := add("commit", []byte("tree "+baseTree+"\n\nbase\n"))
	headCommit := add("commit", []byte("tree "+headTree+"\nparent "+baseCommit+"\n\nhead\n"))
	return &reviewRepository{objects: objects}, contextreceipt.Candidate{
		BaseCommit: baseCommit, BaseTree: baseTree, HeadCommit: headCommit, HeadTree: headTree,
	}, acceptedSpec, reviewPolicy
}

type gitTreeFixtureEntry struct{ mode, name, oid string }

func gitTreeBody(t *testing.T, entries []gitTreeFixtureEntry) []byte {
	t.Helper()
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	var body bytes.Buffer
	for _, entry := range entries {
		oid, err := hex.DecodeString(entry.oid)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&body, "%s %s", entry.mode, entry.name)
		body.WriteByte(0)
		body.Write(oid)
	}
	return body.Bytes()
}

func testGitObjectOID(kind string, content []byte) string {
	preimage := append([]byte(fmt.Sprintf("%s %d\x00", kind, len(content))), content...)
	sum := sha1.Sum(preimage)
	return hex.EncodeToString(sum[:])
}

func reviewBuilderReceiptFixture(t *testing.T, candidate contextreceipt.Candidate) (contextreceipt.Receipt, []byte) {
	t.Helper()
	output := []byte("builder evidence\n")
	evidence := contextreceipt.Evidence{CommandID: "verify", Argv: []string{"make", "verify"}, ExitCode: 0, Verdict: countersign.VerdictProven, OutputDigest: rawDigestForTest(output)}
	receipt := reviewReceiptBase(t, candidate)
	receipt.Role = contextreceipt.RoleBuilder
	receipt.Evidence = []contextreceipt.Evidence{evidence}
	receipt.ReviewInputs = []contextreceipt.ReviewInput{}
	encoded, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		t.Fatalf("EncodeReceipt(builder) error = %v", err)
	}
	decoded, err := contextreceipt.DecodeReceipt(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeReceipt(builder) error = %v", err)
	}
	return decoded, encoded
}

func reviewReceiptFixture(t *testing.T, candidate contextreceipt.Candidate, builderDigest string, inputs []contextreceipt.ReviewInput) (contextreceipt.Receipt, []byte) {
	t.Helper()
	receipt := reviewReceiptBase(t, candidate)
	receipt.Role = contextreceipt.RoleReviewer
	receipt.Evidence = []contextreceipt.Evidence{}
	receipt.ReviewInputs = append([]contextreceipt.ReviewInput(nil), inputs...)
	receipt.ReviewOf = []string{builderDigest}
	encoded, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		t.Fatalf("EncodeReceipt(reviewer) error = %v", err)
	}
	decoded, err := contextreceipt.DecodeReceipt(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeReceipt(reviewer) error = %v", err)
	}
	return decoded, encoded
}

func reviewReceiptBase(t *testing.T, candidate contextreceipt.Candidate) contextreceipt.Receipt {
	t.Helper()
	revision := []contextevent.Revision{{
		Schema: contextevent.RevisionSchemaID, ManifestRevision: 0, ManifestDigest: testDigestA,
		FirstGlobalSequence: 1, TerminalGlobalSequence: 2, TerminalSourceSequence: 2,
		TerminalKind: contextevent.KindExecutionResult, EventRoot: testDigestB,
	}}
	root, err := contextevent.EventChainRoot(revision)
	if err != nil {
		t.Fatal(err)
	}
	claim := gp.PrincipalClaim{TrustSource: "ci-runner", Subject: "reviewer@example.com"}
	principalID, err := gp.CanonicalPrincipalID(claim.TrustSource, claim.Subject)
	if err != nil {
		t.Fatal(err)
	}
	return contextreceipt.Receipt{
		Schema: contextreceipt.SchemaID, Authority: contextreceipt.AuthorityAuthoritative,
		ManifestDigest: testDigestA, DispatchDigest: testDigestB, ATCRunway: "/runway/review",
		ExecutionWorkspaceRequestDigest: testDigestA, ExecutionWorkspaceID: "workspace-review",
		InputCommit: candidate.BaseCommit, InputTree: candidate.BaseTree,
		OutputCommit: candidate.HeadCommit, OutputTree: candidate.HeadTree, Clean: true,
		RevisionSegments: revision, EventChainRoot: root, TerminalManifestRevision: 0,
		TerminalSourceSequence: 2, TerminalGlobalSequence: 2,
		Expansions: []contextreceipt.Expansion{}, Obligations: []contextreceipt.Obligation{},
		RunnerPrincipalResolution: gp.PrincipalResolution{
			Claim: claim, PrincipalID: principalID, State: gp.ResolutionAuthenticated,
			Witnesses: []gp.Witness{{Code: "trust-subject-verified", SourceID: claim.TrustSource, EvidenceDigest: testDigestA}},
		},
		Adapter: contextevent.AdapterCodex, AdapterVersion: "1.2.3",
	}
}

func evidenceResultBytes(t *testing.T, summary contextreceipt.Evidence, output []byte) []byte {
	t.Helper()
	encoded, err := EncodeEvidenceResult(EvidenceResult{
		Schema: EvidenceResultSchemaID, CommandID: summary.CommandID, Argv: append([]string(nil), summary.Argv...),
		ExitCode: summary.ExitCode, Verdict: summary.Verdict, Output: append([]byte(nil), output...),
		OutputDigest: rawDigestForTest(output),
	})
	if err != nil {
		t.Fatalf("EncodeEvidenceResult() error = %v", err)
	}
	decoded, err := DecodeEvidenceResult(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEvidenceResult() error = %v", err)
	}
	if decoded.Digest == "" {
		t.Fatal("decoded evidence result has empty self digest")
	}
	return encoded
}

func reviewAdjudicationFixture(t *testing.T, candidate contextreceipt.Candidate, r0ReceiptDigest string) []byte {
	t.Helper()
	detailBytes := []byte(`{"decision":"accepted"}`)
	claim := gp.PrincipalClaim{TrustSource: "ci-runner", Subject: "owner@example.com"}
	principalID, err := gp.CanonicalPrincipalID(claim.TrustSource, claim.Subject)
	if err != nil {
		t.Fatal(err)
	}
	payloadSchema, err := contextevent.PayloadSchema(contextevent.KindAdjudication)
	if err != nil {
		t.Fatal(err)
	}
	event := contextevent.Event{
		Schema: contextevent.EventSchemaID, SourceSequence: 3, Flight: "flight-review", Lane: "review", Epoch: "epoch-1",
		ManifestRevision: 0, ManifestDigest: testDigestA, Session: "session-r0", ATCRunway: "/runway/review",
		ExecutionWorkspaceID: "workspace-r0", CandidateCommit: candidate.HeadCommit, CandidateTree: candidate.HeadTree,
		Adapter: contextevent.AdapterCodex, AdapterVersion: "1.2.3", OccurredAt: "2026-08-29T12:00:00Z",
		Kind: contextevent.KindAdjudication, PayloadSchema: payloadSchema,
		Payload: &contextevent.AdjudicationPayload{
			Schema: payloadSchema, FindingOrDeviationID: "finding-1",
			PrincipalResolution: gp.PrincipalResolution{
				Claim: claim, PrincipalID: principalID, State: gp.ResolutionAuthenticated,
				Witnesses: []gp.Witness{{Code: "trust-subject-verified", SourceID: claim.TrustSource, EvidenceDigest: testDigestA}},
			},
			Decision: "accept", ReasonDigest: testDigestB,
			Detail: contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: rawDigestForTest(detailBytes), RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: detailBytes},
		},
		PriorEventDigest: testDigestA,
	}
	eventBytes, err := contextevent.EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent(adjudication) error = %v", err)
	}
	event, err = contextevent.DecodeEvent(bytes.NewReader(eventBytes))
	if err != nil {
		t.Fatal(err)
	}
	ack := contextevent.EventAck{
		Schema: contextevent.AckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch,
		Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind,
		SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: 42,
	}
	encoded, err := EncodeAdjudication(Adjudication{
		Schema: AdjudicationSchemaID, R0ReceiptDigest: r0ReceiptDigest,
		Rows: []AdjudicationRow{{FindingOrDeviationID: "finding-1", EventBytes: eventBytes, Ack: ack}},
	})
	if err != nil {
		t.Fatalf("EncodeAdjudication() error = %v", err)
	}
	return encoded
}

func rewriteAdjudicationForTest(t *testing.T, raw []byte, mutate func(*contextevent.Event, *contextevent.AdjudicationPayload)) []byte {
	t.Helper()
	adjudication, err := DecodeAdjudication(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	event, err := contextevent.DecodeEvent(bytes.NewReader(adjudication.Rows[0].EventBytes))
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := event.Payload.(*contextevent.AdjudicationPayload)
	if !ok {
		t.Fatalf("adjudication payload type = %T", event.Payload)
	}
	mutate(&event, payload)
	event.EventDigest = ""
	eventBytes, err := contextevent.EncodeEvent(event)
	if err != nil {
		t.Fatalf("encode mutated adjudication event: %v", err)
	}
	event, err = contextevent.DecodeEvent(bytes.NewReader(eventBytes))
	if err != nil {
		t.Fatal(err)
	}
	adjudication.Digest = ""
	adjudication.Rows[0].EventBytes = eventBytes
	adjudication.Rows[0].Ack.Flight = event.Flight
	adjudication.Rows[0].Ack.Lane = event.Lane
	adjudication.Rows[0].Ack.Epoch = event.Epoch
	adjudication.Rows[0].Ack.Session = event.Session
	adjudication.Rows[0].Ack.ManifestRevision = event.ManifestRevision
	adjudication.Rows[0].Ack.Kind = event.Kind
	adjudication.Rows[0].Ack.SourceSequence = event.SourceSequence
	adjudication.Rows[0].Ack.EventDigest = event.EventDigest
	adjudication.Digest, err = canonjson.Digest(adjudication)
	if err != nil {
		t.Fatalf("digest mutated adjudication wrapper: %v", err)
	}
	encoded, err := canonjson.Marshal(adjudication)
	if err != nil {
		t.Fatalf("marshal mutated adjudication wrapper: %v", err)
	}
	return encoded
}

func rewriteAdjudicationReceiptForTest(t *testing.T, raw []byte, digest string) []byte {
	t.Helper()
	adjudication, err := DecodeAdjudication(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	adjudication.R0ReceiptDigest = digest
	adjudication.Digest = ""
	adjudication.Digest, err = canonjson.Digest(adjudication)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := canonjson.Marshal(adjudication)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func rawDigestForTest(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum)
}
