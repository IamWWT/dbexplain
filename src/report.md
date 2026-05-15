
[1m[96m▸ Instances (8)[0m[0m
  [96maiops-sqlite                  [0m  [93msqlite[0m  [2m1 db(s), 3 tables[0m
  [96mvideo-redis                   [0m  [93mredis[0m  [2m1 db(s), 1 tables[0m
  [96mqdrant-test                   [0m  [93mqdrant[0m  [2m1 db(s), 1 tables[0m
  [96maiops-mysql                   [0m  [93mmysql[0m  [2m1 db(s), 2 tables[0m
  [96mopenim-redis                  [0m  [93mredis[0m  [2m1 db(s), 40 tables[0m
  [96mes-test                       [0m  [93melasticsearch[0m  [2m1 db(s), 1 tables[0m
  [96mvideo-pg                      [0m  [93mpostgres[0m  [2m1 db(s), 5 tables[0m
  [96maiops-clickhouse              [0m  [93mclickhouse[0m  [2m2 db(s), 6 tables[0m

[1m[96m▸ aiops-sqlite  /  [1m/home/wwt/Downloads/aigc/proj/agents/aiops/intent-apparatus/data/rules.db[0m[0m[0m

  [1mfailure_log[0m[2m[0m[2m ~3 rows[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname        [0m  [1mtype     [0m  [1mflags       [0m  [1mcomment[0m
  [2m────────────[0m  [2m─────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mid          [0m  [2mINTEGER  [0m  [93mPK[0m   [2m标识符[0m
  session_id    [2mTEXT     [0m                [2m标识符[0m
  raw_input     [2mTEXT     [0m                [2m示例: web-02[0m
  context_json  [2mTEXT     [0m                [2mJSON 数据[0m
  reason        [2mTEXT     [0m                [2m示例: LLM fallback failed[0m
  created_at    [2mTIMESTAMP[0m                [2m示例: 2026-05-09 16:28:43 …[0m

  [1mrules[0m[2m[0m[2m ~21 rows[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname             [0m  [1mtype     [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────────────────[0m  [2m─────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mrule_id          [0m  [2mTEXT     [0m  [93mPK[0m   [2m标识符[0m
  type               [2mTEXT     [0m                [2m类型[0m
  condition_json     [2mTEXT     [0m                [2mJSON 数据[0m
  action_json        [2mTEXT     [0m                [2mJSON 数据[0m
  weight             [2mREAL     [0m                [2m示例: 1[0m
  total_activations  [2mINTEGER  [0m                [2m金额/数量[0m
  successes          [2mINTEGER  [0m                [2m示例: 0[0m
  failures           [2mINTEGER  [0m                [2m示例: 0[0m
  status             [2mTEXT     [0m                [2m状态[0m
  created_by         [2mTEXT     [0m                [2m示例: seed_llm[0m
  birth_version      [2mINTEGER  [0m                [2m示例: 1[0m
  last_used          [2mTEXT     [0m                [2m[0m
  evolution_parent   [2mTEXT     [0m                [2m[0m
  created_at         [2mTIMESTAMP[0m                [2m示例: 2026-05-10 00:13:37 …[0m
  [2mindexes:[0m [2mUNI(rule_id)[0m

  [1msessions[0m[2m[0m[2m ~10 rows[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname        [0m  [1mtype     [0m  [1mflags       [0m  [1mcomment[0m
  [2m────────────[0m  [2m─────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93msession_id  [0m  [2mTEXT     [0m  [93mPK[0m   [2m标识符[0m
  context_json  [2mTEXT     [0m                [2mJSON 数据[0m
  updated_at    [2mTIMESTAMP[0m                [2m时间[0m
  [2mindexes:[0m [2mUNI(session_id)[0m

[1m[96m▸ video-redis  /  [1mdb0[0m[0m[0m

  [1m_server_info[0m[2m[0m[2m[0m[2m[0m  [2mRedis 7.4.7 | memory=1.11M | total_keys=[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
[2m    (no columns)[0m

[1m[96m▸ qdrant-test  /  [1mdefault[0m[0m[0m

  [1mmcp_tools[0m[2m [qdrant][0m[2m ~61 rows[0m[2m[0m  [2mvector collection[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname  [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m──────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  vector  [2mfloat[][0m  [2mNN[0m    [2membedding vector[0m

[1m[96m▸ aiops-mysql  /  [1mtestdb[0m[0m[0m

  [1miplist[0m[2m [InnoDB][0m[2m ~12 rows[0m[2m 16.0KB[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname       [0m  [1mtype        [0m  [1mflags       [0m  [1mcomment[0m
  [2m───────────[0m  [2m────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mID         [0m  [2mint         [0m  [93mPK[0m   [2m标识符[0m
  envlabel     [2mvarchar(100)[0m  [2mNN[0m    [2m产品环境标识[0m
  product      [2mvarchar(100)[0m  [2mNN[0m    [2m产品标识[0m
  subproduct   [2mvarchar(100)[0m                [2m示例: [97 105 111 112 115 …[0m
  hostip       [2mvarchar(100)[0m                [2mIP 地址[0m
  bmcip        [2mvarchar(100)[0m                [2mIP 地址[0m
  device_type  [2mvarchar(20) [0m                [2m类型[0m
  arch         [2mvarchar(20) [0m                [2m示例: [88 56 54][0m
  card         [2mvarchar(50) [0m                [2m显卡型号[0m
  cardtype     [2mvarchar(30) [0m                [2m类型[0m
  cardnumber   [2mint         [0m                [2m示例: 1[0m
  datacenter   [2mvarchar(20) [0m                [2m示例: [120 97][0m
  dcloc        [2mvarchar(20) [0m                [2m示例: [119 116 104 111 109…[0m
  owner        [2mvarchar(100)[0m                [2m机器所属[0m
  isreg        [2mtinyint(1)  [0m  [2mNN[0m    [2m1表示纳入监控，0表示不予监控[0m

  [1mport[0m[2m [InnoDB][0m[2m ~30 rows[0m[2m 16.0KB[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname       [0m  [1mtype        [0m  [1mflags       [0m  [1mcomment[0m
  [2m───────────[0m  [2m────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mID         [0m  [2mint         [0m  [93mPK[0m   [2m标识符[0m
  port         [2mvarchar(100)[0m  [2mNN[0m    [2m示例: [57 52 48 48][0m
  service      [2mvarchar(50) [0m  [2mNN[0m    [2m示例: [110 111 100 101 101…[0m
  protocol     [2mvarchar(100)[0m                [2m协议[0m
  description  [2mvarchar(100)[0m                [2mIP 地址[0m
  isreg        [2mtinyint(1)  [0m                [2m1表示注册到consul,0表示不注册[0m

[1m[96m▸ openim-redis  /  [1mdb0[0m[0m[0m

  [1m_server_info[0m[2m[0m[2m[0m[2m[0m  [2mRedis 7.0.0 | memory=2.19M | total_keys=keys=332,expires=332,avg_ttl=3622000836[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
[2m    (no columns)[0m

  [1mSEQ_USER_READ:si_{hex}_{hex}:{hex}[0m[2m[0m[2m ~30 rows[0m[2m[0m  [2m30 keys, type=[0m[2m  key=SEQ_USER_READ:si_{hex}_{hex}:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mCONVERSATION:{hex}:si_{hex}_{hex}[0m[2m[0m[2m ~27 rows[0m[2m[0m  [2m27 keys, type=[0m[2m  key=CONVERSATION:{hex}:si_{hex}_{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mMSG_CACHE:si_{hex}_{hex}:{id}[0m[2m[0m[2m ~16 rows[0m[2m[0m  [2m16 keys, type=[0m[2m  key=MSG_CACHE:si_{hex}_{hex}:{id}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mSEQ_USER_MIN:si_{hex}_{hex}:{hex}[0m[2m[0m[2m ~15 rows[0m[2m[0m  [2m15 keys, type=[0m[2m  key=SEQ_USER_MIN:si_{hex}_{hex}:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mMALLOC_SEQ:si_{hex}_{hex}[0m[2m[0m[2m ~15 rows[0m[2m[0m  [2m15 keys, type=[0m[2m  key=MALLOC_SEQ:si_{hex}_{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname[0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  CURR  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  LAST  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  TIME  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mSEQ_USER_MAX:si_{hex}_{hex}:{hex}[0m[2m[0m[2m ~15 rows[0m[2m[0m  [2m15 keys, type=[0m[2m  key=SEQ_USER_MAX:si_{hex}_{hex}:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mMALLOC_SEQ:n_{hex}_{hex}[0m[2m[0m[2m ~14 rows[0m[2m[0m  [2m14 keys, type=[0m[2m  key=MALLOC_SEQ:n_{hex}_{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname[0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  TIME  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  CURR  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  LAST  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mUID_PID_TOKEN_STATUS:{hex}:[0m[2m[0m[2m ~13 rows[0m[2m[0m  [2m13 keys, type=[0m[2m  key=UID_PID_TOKEN_STATUS:{hex}:[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname                                    [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m────────────────────────────────────────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mUSER_INFO:{hex}[0m[2m[0m[2m ~12 rows[0m[2m[0m  [2m12 keys, type=[0m[2m  key=USER_INFO:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mCONVERSATION_USER_MAX:{hex}[0m[2m[0m[2m ~11 rows[0m[2m[0m  [2m11 keys, type=[0m[2m  key=CONVERSATION_USER_MAX:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mONLINE:{hex}[0m[2m[0m[2m ~11 rows[0m[2m[0m  [2m11 keys, type=[0m[2m  key=ONLINE:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname  [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m──────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  member  [2mstring [0m  [2mNN[0m    [2m[0m
  score   [2mfloat64[0m  [2mNN[0m    [2m[0m

  [1mCONVERSATION_IDS:{hex}[0m[2m[0m[2m ~11 rows[0m[2m[0m  [2m11 keys, type=[0m[2m  key=CONVERSATION_IDS:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mFRIEND_MAX_VERSION:{hex}[0m[2m[0m[2m ~11 rows[0m[2m[0m  [2m11 keys, type=[0m[2m  key=FRIEND_MAX_VERSION:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mGROUP_JOIN_MAX_VERSION:{hex}[0m[2m[0m[2m ~11 rows[0m[2m[0m  [2m11 keys, type=[0m[2m  key=GROUP_JOIN_MAX_VERSION:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mFRIEND_IDS:{hex}[0m[2m[0m[2m ~10 rows[0m[2m[0m  [2m10 keys, type=[0m[2m  key=FRIEND_IDS:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mSEQ_USER_READ:sg_{hex}:{hex}[0m[2m[0m[2m ~10 rows[0m[2m[0m  [2m10 keys, type=[0m[2m  key=SEQ_USER_READ:sg_{hex}:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mJOIN_GROUPS_KEY:{hex}[0m[2m[0m[2m ~10 rows[0m[2m[0m  [2m10 keys, type=[0m[2m  key=JOIN_GROUPS_KEY:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mCONVERSATION:{hex}:sg_{hex}[0m[2m[0m[2m ~10 rows[0m[2m[0m  [2m10 keys, type=[0m[2m  key=CONVERSATION:{hex}:sg_{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mNOT_NOTIFY_CONVERSATION_IDS:{hex}[0m[2m[0m[2m ~10 rows[0m[2m[0m  [2m10 keys, type=[0m[2m  key=NOT_NOTIFY_CONVERSATION_IDS:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mGROUP_MEMBER_INFO:{hex}-{hex}[0m[2m[0m[2m ~10 rows[0m[2m[0m  [2m10 keys, type=[0m[2m  key=GROUP_MEMBER_INFO:{hex}-{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mSEQ_USER_MIN:sg_{hex}:{hex}[0m[2m[0m[2m ~9 rows[0m[2m[0m  [2m9 keys, type=[0m[2m  key=SEQ_USER_MIN:sg_{hex}:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mPINNED_CONVERSATION_IDS:{hex}[0m[2m[0m[2m ~9 rows[0m[2m[0m  [2m9 keys, type=[0m[2m  key=PINNED_CONVERSATION_IDS:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mSEQ_USER_MAX:sg_{hex}:{hex}[0m[2m[0m[2m ~9 rows[0m[2m[0m  [2m9 keys, type=[0m[2m  key=SEQ_USER_MAX:sg_{hex}:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mSEND_MSG_FAILED_FLAG:{hex}-{hex}[0m[2m[0m[2m ~8 rows[0m[2m[0m  [2m8 keys, type=[0m[2m  key=SEND_MSG_FAILED_FLAG:{hex}-{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname   [0m  [1mtype    [0m  [1mflags       [0m  [1mcomment[0m
  [2m───────[0m  [2m────────[0m  [2m────────────[0m  [2m────────────────────[0m
  (value)  [2minteger [0m  [2mNN[0m    [2m2[0m
  ttl      [2mduration[0m  [2mNN[0m    [2m14h32m53s[0m

  [1mMSG_CACHE:si_{hex}_{hex}:5[0m[2m[0m[2m ~7 rows[0m[2m[0m  [2m7 keys, type=[0m[2m  key=MSG_CACHE:si_{hex}_{hex}:5[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mBLACK_IDS:{hex}[0m[2m[0m[2m ~4 rows[0m[2m[0m  [2m4 keys, type=[0m[2m  key=BLACK_IDS:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mMSG_CACHE:n_{hex}_{hex}:{id}[0m[2m[0m[2m ~2 rows[0m[2m[0m  [2m2 keys, type=[0m[2m  key=MSG_CACHE:n_{hex}_{hex}:{id}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mCHAT_UID_TOKEN_STATUS:{hex}[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=CHAT_UID_TOKEN_STATUS:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname                                    [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m────────────────────────────────────────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mUID_PID_TOKEN_STATUS:{hex}:Web[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=UID_PID_TOKEN_STATUS:{hex}:Web[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname                                    [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m────────────────────────────────────────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mOBJECT:{hex}/msg_picture_{hex}.png:minio[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=OBJECT:{hex}/msg_picture_{hex}.png:minio[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mGROUP_MEMBER_MAX_VERSION:{hex}[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=GROUP_MEMBER_MAX_VERSION:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mMSG_CACHE:sg_{hex}:6[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=MSG_CACHE:sg_{hex}:6[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mGROUP_ROLE_LEVEL_MEMBER_IDS:{hex}-{id}[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=GROUP_ROLE_LEVEL_MEMBER_IDS:{hex}-{id}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mGROUP_MEMBER_NUM_CACHE:{hex}[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=GROUP_MEMBER_NUM_CACHE:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mUSER_INFO:imAdmin[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=USER_INFO:imAdmin[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mCHAT_UID_TOKEN_STATUS:imAdmin[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=CHAT_UID_TOKEN_STATUS:imAdmin[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname                                    [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m────────────────────────────────────────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ey…  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mGROUP_INFO:{hex}[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=GROUP_INFO:{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname [0m  [1mtype[0m  [1mflags       [0m  [1mcomment[0m
  [2m─────[0m  [2m────[0m  [2m────────────[0m  [2m────────────────────[0m
  value  [2mjson[0m  [2mNN[0m    [2mhash field[0m

  [1mMALLOC_SEQ:n_{hex}[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=MALLOC_SEQ:n_{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname[0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  CURR  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  LAST  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  TIME  [2minteger[0m  [2mNN[0m    [2mhash field[0m

  [1mMALLOC_SEQ:sg_{hex}[0m[2m[0m[2m ~1 rows[0m[2m[0m  [2m1 keys, type=[0m[2m  key=MALLOC_SEQ:sg_{hex}[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname[0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  CURR  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  LAST  [2minteger[0m  [2mNN[0m    [2mhash field[0m
  TIME  [2minteger[0m  [2mNN[0m    [2mhash field[0m

[1m[96m▸ es-test  /  [1melasticsearch[0m[0m[0m

  [1mrunbooks[0m[2m [elasticsearch][0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname          [0m  [1mtype   [0m  [1mflags       [0m  [1mcomment[0m
  [2m──────────────[0m  [2m───────[0m  [2m────────────[0m  [2m────────────────────[0m
  resolution      [2mtext   [0m  [2mNN[0m    [2mes field[0m
  created_by      [2mkeyword[0m  [2mNN[0m    [2mes field[0m
  last_updated    [2mdate   [0m  [2mNN[0m    [2mes field[0m
  reviewed_at     [2mdate   [0m  [2mNN[0m    [2mes field[0m
  steps           [2mnested [0m  [2mNN[0m    [2mes field[0m
  category        [2mkeyword[0m  [2mNN[0m    [2mes field[0m
  ingested_at     [2mdate   [0m  [2mNN[0m    [2mes field[0m
  owner_team      [2mkeyword[0m  [2mNN[0m    [2mes field[0m
  prerequisites   [2mkeyword[0m  [2mNN[0m    [2mes field[0m
  severity        [2mkeyword[0m  [2mNN[0m    [2mes field[0m
  tags            [2mkeyword[0m  [2mNN[0m    [2mes field[0m
  symptoms        [2mtext   [0m  [2mNN[0m    [2mes field[0m
  escalation      [2m<nil>  [0m  [2mNN[0m    [2mes field[0m
  title           [2mtext   [0m  [2mNN[0m    [2mes field[0m
  rollback        [2mtext   [0m  [2mNN[0m    [2mes field[0m
  alert_triggers  [2mkeyword[0m  [2mNN[0m    [2mes field[0m
  reviewed_by     [2mkeyword[0m  [2mNN[0m    [2mes field[0m

[1m[96m▸ video-pg  /  [1mvideomon[0m[0m[0m

  [1mabnormal_events[0m[2m[0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname           [0m  [1mtype                       [0m  [1mflags       [0m  [1mcomment[0m
  [2m───────────────[0m  [2m───────────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mid             [0m  [2muuid                       [0m  [93mPK[0m   [2m标识符[0m
  camera_id        [2muuid                       [0m  [2mNN[0m    [2m标识符[0m
  event_time       [2mtimestamp without time zone[0m  [2mNN[0m    [2m时间[0m
  event_type       [2mcharacter varying(50)      [0m  [2mNN[0m    [2m类型[0m
  description      [2mtext                       [0m  [2mNN[0m    [2mIP 地址[0m
  severity         [2minteger                    [0m                [2m1-5, 5最高[0m
  video_clip_path  [2mcharacter varying(500)     [0m                [2mIP 地址[0m
  screenshot_path  [2mcharacter varying(500)     [0m                [2m示例: http://localhost:900…[0m
  start_time       [2mtimestamp without time zone[0m                [2m时间[0m
  end_time         [2mtimestamp without time zone[0m                [2m时间[0m
  is_handled       [2mboolean                    [0m                [2m标志位[0m
  handled_by       [2muuid                       [0m                [2m示例: NULL[0m
  handled_at       [2mtimestamp without time zone[0m                [2m示例: NULL[0m
  extra_metadata   [2mjson                       [0m                [2mJSON 数据[0m
  created_at       [2mtimestamp without time zone[0m                [2m示例: 2026-02-19 08:55:25.…[0m
  [2mindexes:[0m [2mUNI(id)  IDX(event_time)[0m

  [1malert_logs[0m[2m[0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname         [0m  [1mtype                       [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────────────[0m  [2m───────────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mid           [0m  [2muuid                       [0m  [93mPK[0m   [2m标识符[0m
  event_id       [2muuid                       [0m  [2mNN[0m    [2m标识符[0m
  alert_time     [2mtimestamp without time zone[0m  [2mNN[0m    [2m时间[0m
  alert_message  [2mtext                       [0m  [2mNN[0m    [2m示例: 监控画面显示�…[0m
  alert_channel  [2mcharacter varying(50)      [0m                [2m示例: system[0m
  is_notified    [2mboolean                    [0m                [2m标志位[0m
  notified_at    [2mtimestamp without time zone[0m                [2m示例: 2026-02-19 08:55:25.…[0m
  created_at     [2mtimestamp without time zone[0m                [2m示例: 2026-02-19 08:55:25.…[0m
  [2mindexes:[0m [2mUNI(id)[0m

  [1mcameras[0m[2m[0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname              [0m  [1mtype                       [0m  [1mflags       [0m  [1mcomment[0m
  [2m──────────────────[0m  [2m───────────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mid                [0m  [2muuid                       [0m  [93mPK[0m   [2m标识符[0m
  name                [2mcharacter varying(100)     [0m  [2mNN[0m    [2m名称[0m
  rtsp_url            [2mcharacter varying(500)     [0m  [92mUNI[0m [2mNN[0m  [2mURL[0m
  location            [2mcharacter varying(200)     [0m                [2m示例: 客厅[0m
  status              [2minteger                    [0m                [2m0:离线 1:在线[0m
  config              [2mjson                       [0m                [2mJSON 数据[0m
  last_heartbeat      [2mtimestamp without time zone[0m                [2m示例: 2026-02-19 17:05:43.…[0m
  health_check_count  [2minteger                    [0m                [2m示例: 0[0m
  created_at          [2mtimestamp without time zone[0m                [2m示例: 2026-02-19 08:55:21.…[0m
  updated_at          [2mtimestamp without time zone[0m                [2m时间[0m
  [2mindexes:[0m [2mUNI(id)  UNI(rtsp_url)[0m

  [1mreports[0m[2m[0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname       [0m  [1mtype                       [0m  [1mflags       [0m  [1mcomment[0m
  [2m───────────[0m  [2m───────────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mid         [0m  [2muuid                       [0m  [93mPK[0m   [2m标识符[0m
  name         [2mcharacter varying(200)     [0m  [2mNN[0m    [2m名称[0m
  type         [2mcharacter varying(50)      [0m                [2m类型[0m
  start_time   [2mtimestamp without time zone[0m  [2mNN[0m    [2m时间[0m
  end_time     [2mtimestamp without time zone[0m  [2mNN[0m    [2m时间[0m
  camera_ids   [2mjson                       [0m                [2mJSON 数据[0m
  event_types  [2mjson                       [0m                [2m类型[0m
  content      [2mjson                       [0m                [2mJSON 数据[0m
  summary      [2mtext                       [0m                [2m示例: NULL[0m
  file_path    [2mcharacter varying(500)     [0m                [2m示例: NULL[0m
  status       [2mcharacter varying(20)      [0m                [2m状态[0m
  progress     [2minteger                    [0m                [2m示例: 0[0m
  created_by   [2muuid                       [0m                [2m示例: NULL[0m
  created_at   [2mtimestamp without time zone[0m                [2m示例: 2026-02-19 09:26:00.…[0m
  [2mindexes:[0m [2mUNI(id)[0m

  [1mvideo_descriptions[0m[2m[0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname          [0m  [1mtype                       [0m  [1mflags       [0m  [1mcomment[0m
  [2m──────────────[0m  [2m───────────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mid            [0m  [2muuid                       [0m  [93mPK[0m   [2m标识符[0m
  camera_id       [2muuid                       [0m  [2mNN[0m    [2m标识符[0m
  timestamp       [2mtimestamp without time zone[0m  [2mNN[0m    [2m时间[0m
  description     [2mtext                       [0m  [2mNN[0m    [2mIP 地址[0m
  embedding       [2mvector(1024)               [0m                [2m示例: NULL[0m
  is_abnormal     [2mboolean                    [0m                [2m标志位[0m
  frame_path      [2mcharacter varying(500)     [0m                [2m[0m
  extra_metadata  [2mjson                       [0m                [2mJSON 数据[0m
  created_at      [2mtimestamp without time zone[0m                [2m示例: 2026-02-19 08:55:24.…[0m
  [2mindexes:[0m [2mUNI(id)  IDX("timestamp")[0m

[1m[96m▸ aiops-clickhouse  /  [1mai_obs[0m[0m[0m

  [1magent_events[0m[2m [MergeTree][0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname             [0m  [1mtype                  [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────────────────[0m  [2m──────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mtimestamp        [0m  [2mDateTime64(3)         [0m  [93mPK[0m [94mSORT[0m [92mPART[0m  [2m[0m
  [93mtrace_id         [0m  [2mString                [0m  [93mPK[0m [94mSORT[0m  [2m[0m
  span_id            [2mString                [0m  [2mNN[0m    [2m[0m
  parent_span_id     [2mString                [0m  [2mNN[0m    [2m[0m
  [93magent_id         [0m  [2mString                [0m  [93mPK[0m [94mSORT[0m  [2m[0m
  service_name       [2mString                [0m  [2mNN[0m    [2m[0m
  environment        [2mLowCardinality(String)[0m  [2mNN[0m    [2m[0m
  event_type         [2mLowCardinality(String)[0m  [2mNN[0m    [2m[0m
  event_name         [2mString                [0m  [2mNN[0m    [2m[0m
  llm_model          [2mString                [0m  [2mNN[0m    [2m[0m
  prompt_tokens      [2mUInt32                [0m  [2mNN[0m    [2m[0m
  completion_tokens  [2mUInt32                [0m  [2mNN[0m    [2m[0m
  total_tokens       [2mUInt32                [0m  [2mNN[0m    [2m[0m
  tool_name          [2mString                [0m  [2mNN[0m    [2m[0m
  latency_ms         [2mUInt32                [0m  [2mNN[0m    [2m[0m
  status             [2mLowCardinality(String)[0m  [2mNN[0m    [2m[0m
  payload            [2mString                [0m  [2mNN[0m    [2m[0m
  [2mPARTITION BY[0m [94mtoDate(timestamp)[0m
  [2mORDER BY[0m    [94magent_id, trace_id, timestamp[0m

  [1mmcp_server_registry[0m[2m [ReplacingMergeTree][0m[2m ~1 rows[0m[2m 1.3KB[0m  [2mMCP Server 注册表 — Consul 的持久化备份和扩展元数据[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname             [0m  [1mtype                  [0m  [1mflags       [0m  [1mcomment[0m
  [2m─────────────────[0m  [2m──────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mserver_name      [0m  [2mString                [0m  [93mPK[0m [94mSORT[0m  [2mMCP Server 名称, 如 mcp.ops.monitor.prometheus[0m
  domain             [2mLowCardinality(String)[0m  [2mNN[0m    [2m工具域: monitor|messaging|logs|execution|dashboard|browser[0m
  mcp_url            [2mString                [0m  [2mNN[0m    [2mMCP 协议端点, 如 http://192.168.0.127:9470/mcp[0m
  protocol           [2mLowCardinality(String)[0m  [2mNN[0m    [2m传输协议: http|streamable-http|sse[0m
  max_risk_level     [2mLowCardinality(String)[0m  [2mNN[0m    [2m最高风险等级: readonly|low|high|critical[0m
  tool_count         [2mUInt16                [0m  [2mNN[0m    [2m当前提供的工具数量[0m
  version            [2mString                [0m  [2mNN[0m    [2mServer 版本号[0m
  status             [2mLowCardinality(String)[0m  [2mNN[0m    [2mactive|inactive|draining[0m
  health_last_check  [2mDateTime              [0m  [2mNN[0m    [2m最后一次健康检查时间[0m
  health_status      [2mLowCardinality(String)[0m  [2mNN[0m    [2mhealthy|unhealthy|unknown[0m
  tags               [2mArray(String)         [0m  [2mNN[0m    [2m标签: [mcp, ops, prod, grpc][0m
  created_at         [2mDateTime              [0m  [2mNN[0m    [2m[0m
  updated_at         [2mDateTime              [0m  [2mNN[0m    [2m[0m
  [2mORDER BY[0m    [94mserver_name[0m

  [1motel_traces[0m[2m [MergeTree][0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname              [0m  [1mtype                                    [0m  [1mflags       [0m  [1mcomment[0m
  [2m──────────────────[0m  [2m────────────────────────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mTimestamp         [0m  [2mDateTime64(9)                           [0m  [93mPK[0m [94mSORT[0m [92mPART[0m  [2m[0m
  TraceId             [2mString                                  [0m  [2mNN[0m    [2m[0m
  SpanId              [2mString                                  [0m  [2mNN[0m    [2m[0m
  ParentSpanId        [2mString                                  [0m  [2mNN[0m    [2m[0m
  TraceState          [2mString                                  [0m  [2mNN[0m    [2m[0m
  [93mSpanName          [0m  [2mLowCardinality(String)                  [0m  [93mPK[0m [94mSORT[0m  [2m[0m
  SpanKind            [2mLowCardinality(String)                  [0m  [2mNN[0m    [2m[0m
  [93mServiceName       [0m  [2mLowCardinality(String)                  [0m  [93mPK[0m [94mSORT[0m  [2m[0m
  ResourceAttributes  [2mMap(LowCardinality(String), String)     [0m  [2mNN[0m    [2m[0m
  ScopeName           [2mString                                  [0m  [2mNN[0m    [2m[0m
  ScopeVersion        [2mString                                  [0m  [2mNN[0m    [2m[0m
  SpanAttributes      [2mMap(LowCardinality(String), String)     [0m  [2mNN[0m    [2m[0m
  Duration            [2mUInt64                                  [0m  [2mNN[0m    [2m[0m
  StatusCode          [2mLowCardinality(String)                  [0m  [2mNN[0m    [2m[0m
  StatusMessage       [2mString                                  [0m  [2mNN[0m    [2m[0m
  Events.Timestamp    [2mArray(DateTime64(9))                    [0m  [2mNN[0m    [2m[0m
  Events.Name         [2mArray(LowCardinality(String))           [0m  [2mNN[0m    [2m[0m
  Events.Attributes   [2mArray(Map(LowCardinality(String), Strin…[0m  [2mNN[0m    [2m[0m
  Links.TraceId       [2mArray(String)                           [0m  [2mNN[0m    [2m[0m
  Links.SpanId        [2mArray(String)                           [0m  [2mNN[0m    [2m[0m
  Links.TraceState    [2mArray(String)                           [0m  [2mNN[0m    [2m[0m
  Links.Attributes    [2mArray(Map(LowCardinality(String), Strin…[0m  [2mNN[0m    [2m[0m
  [2mPARTITION BY[0m [94mtoDate(Timestamp)[0m
  [2mORDER BY[0m    [94mServiceName, SpanName, toDateTime(Timestamp)[0m

  [1motel_traces_trace_id_ts[0m[2m [MergeTree][0m[2m[0m[2m[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname   [0m  [1mtype    [0m  [1mflags       [0m  [1mcomment[0m
  [2m───────[0m  [2m────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mTraceId[0m  [2mString  [0m  [93mPK[0m [94mSORT[0m  [2m[0m
  [93mStart  [0m  [2mDateTime[0m  [93mPK[0m [94mSORT[0m [92mPART[0m  [2m[0m
  End      [2mDateTime[0m  [2mNN[0m    [2m[0m
  [2mPARTITION BY[0m [94mtoDate(Start)[0m
  [2mORDER BY[0m    [94mTraceId, Start[0m

  [1mtool_call_log[0m[2m [MergeTree][0m[2m[0m[2m[0m  [2mTool 调用观测日志 — 形成注册→调用→观测→优化闭环[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname          [0m  [1mtype                  [0m  [1mflags       [0m  [1mcomment[0m
  [2m──────────────[0m  [2m──────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  [93mtimestamp     [0m  [2mDateTime64(3)         [0m  [93mPK[0m [94mSORT[0m [92mPART[0m  [2m[0m
  trace_id        [2mString                [0m  [2mNN[0m    [2mOTel trace ID[0m
  [93magent_id      [0m  [2mLowCardinality(String)[0m  [93mPK[0m [94mSORT[0m  [2m调用方 Agent[0m
  [93mtool_id       [0m  [2mString                [0m  [93mPK[0m [94mSORT[0m  [2m被调工具 ID[0m
  tool_name       [2mString                [0m  [2mNN[0m    [2m工具名称[0m
  server_name     [2mString                [0m  [2mNN[0m    [2mMCP Server[0m
  latency_ms      [2mUInt32                [0m  [2mNN[0m    [2m调用延迟 (ms)[0m
  status          [2mLowCardinality(String)[0m  [2mNN[0m    [2msuccess|error|timeout[0m
  error_message   [2mString                [0m  [2mNN[0m    [2m错误信息 (失败时)[0m
  input_params    [2mString                [0m  [2mNN[0m    [2m调用参数 (截断)[0m
  output_summary  [2mString                [0m  [2mNN[0m    [2m返回摘要 (截断)[0m
  token_count     [2mUInt32                [0m  [2mNN[0m    [2m工具返回内容 token 数[0m
  [2mPARTITION BY[0m [94mtoDate(timestamp)[0m
  [2mORDER BY[0m    [94magent_id, tool_id, timestamp[0m

  [1mtool_registry[0m[2m [ReplacingMergeTree][0m[2m ~61 rows[0m[2m 9.1KB[0m  [2mTool Registry — MCP 工具的 Schema 存储和生命周期治理[0m[2m[0m
[2m────────────────────────────────────────────────────────────────────────[0m
  [1mname          [0m  [1mtype                  [0m  [1mflags       [0m  [1mcomment[0m
  [2m──────────────[0m  [2m──────────────────────[0m  [2m────────────[0m  [2m────────────────────[0m
  tool_id         [2mString                [0m  [2mNN[0m    [2m工具唯一 ID, 如 tool_prom_query_metric[0m
  [93mtool_name     [0m  [2mString                [0m  [93mPK[0m [94mSORT[0m  [2m工具名称, 如 mcp_prometheus_execute_query[0m
  [93mserver_name   [0m  [2mString                [0m  [93mPK[0m [94mSORT[0m  [2m所属 MCP Server, 如 mcp.ops.monitor.prometheus[0m
  description     [2mString                [0m  [2mNN[0m    [2m工具描述 (人工可读)[0m
  when_to_use     [2mString                [0m  [2mNN[0m    [2m使用场景说明 (帮助 LLM 判断何时调用)[0m
  input_schema    [2mString                [0m  [2mNN[0m    [2m输入 JSON Schema (完整)[0m
  output_schema   [2mString                [0m  [2mNN[0m    [2m输出 JSON Schema[0m
  risk_level      [2mLowCardinality(String)[0m  [2mNN[0m    [2m风险等级: readonly|low|high|critical[0m
  owner           [2mString                [0m  [2mNN[0m    [2m负责人/团队[0m
  domain          [2mLowCardinality(String)[0m  [2mNN[0m    [2m工具域: monitor|messaging|logs|execution|dashboard|browser[0m
  status          [2mLowCardinality(String)[0m  [2mNN[0m    [2mdraft|reviewing|approved|published|deprecated|offline[0m
  version         [2mString                [0m  [2mNN[0m    [2m[0m
  tags            [2mArray(String)         [0m  [2mNN[0m    [2m标签: [prometheus, query, metrics][0m
  usage_count     [2mUInt64                [0m  [2mNN[0m    [2m累计调用次数 (Observation 更新)[0m
  last_called_at  [2mDateTime              [0m  [2mNN[0m    [2m最近一次调用时间[0m
  avg_latency_ms  [2mFloat32               [0m  [2mNN[0m    [2m平均延迟 (ms)[0m
  success_rate    [2mFloat32               [0m  [2mNN[0m    [2m调用成功率 (0.0-1.0)[0m
  created_at      [2mDateTime              [0m  [2mNN[0m    [2m[0m
  updated_at      [2mDateTime              [0m  [2mNN[0m    [2m[0m
  [2mORDER BY[0m    [94mserver_name, tool_name[0m

[1m[96m▸ aiops-clickhouse  /  [1mdefault[0m[0m[0m

[1m[96m▸ Relationships  (3 explicit FK, 1 inferred)[0m[0m
  [96mvideo-pg/videomon.abnormal_events(camera_id)[0m [92m──FK──▶[0m [96mvideo-pg/videomon.cameras(id)[0m[2m[0m
  [96mvideo-pg/videomon.alert_logs(event_id)[0m [92m──FK──▶[0m [96mvideo-pg/videomon.abnormal_events(id)[0m[2m[0m
  [96mvideo-pg/videomon.video_descriptions(camera_id)[0m [92m──FK──▶[0m [96mvideo-pg/videomon.cameras(id)[0m[2m[0m
  [96maiops-sqlite//home/wwt/Downloads/aigc/proj/agents/aiops/intent-apparatus/data/rules.db.failure_log(session_id)[0m [2m~~?~~~▶[0m [96maiops-sqlite//home/wwt/Downloads/aigc/proj/agents/aiops/intent-apparatus/data/rules.db.sessions(session_id)[0m[2m (inferred, 85%)[0m

[1m[96m▸ Clusters (55)[0m[0m
  [1m4-table cluster[0m
    • [2mvideo-pg/videomon/abnormal_events[0m
    • [2mvideo-pg/videomon/alert_logs[0m
    • [2mvideo-pg/videomon/cameras[0m
    • [2mvideo-pg/videomon/video_descriptions[0m
  [1m2-table cluster[0m
    • [2maiops-sqlite//home/wwt/Downloads/aigc/proj/agents/aiops/intent-apparatus/data/rules.db/failure_log[0m
    • [2maiops-sqlite//home/wwt/Downloads/aigc/proj/agents/aiops/intent-apparatus/data/rules.db/sessions[0m

[1m[96m▸ Issues (5)[0m[0m
  [94mℹ[0m [2maiops-mysql/testdb/iplist[0m  no timestamp column — audit trail gap
  [94mℹ[0m [2maiops-mysql/testdb/port[0m  no timestamp column — audit trail gap
  [93m⚠[0m [2mvideo-pg/videomon/abnormal_events[0m  FK column "camera_id" has no index — full scan risk
  [93m⚠[0m [2mvideo-pg/videomon/alert_logs[0m  FK column "event_id" has no index — full scan risk
  [93m⚠[0m [2mvideo-pg/videomon/video_descriptions[0m  FK column "camera_id" has no index — full scan risk

