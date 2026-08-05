// Package ec implémente le protocole aMule External Connections (EC), celui que
// parle le démon amuled sur son port de contrôle.
//
// Les constantes de ce fichier sont transcrites d'après les en-têtes de
// référence d'aMule (ECCodes.h et ECTagTypes.h). Ce sont des faits d'interface :
// des numéros imposés par le protocole, sans lesquels aucun dialogue n'est
// possible. Aucun code d'aMule n'est repris ici, seules les valeurs numériques
// le sont, et la documentation est écrite de zéro.
package ec

import "fmt"

// ProtocolVersion est la version du protocole parlée par ce client.
//
// amuled compare cette valeur à la sienne de façon stricte : ce n'est pas un
// « au moins cette version », c'est une égalité. Un démon compilé avec une autre
// version refuse l'authentification, quel que soit le sens de l'écart. Un tel
// refus doit donc être signalé explicitement à l'utilisateur (« version de
// protocole incompatible ») et jamais contourné en renvoyant la version
// annoncée par le serveur.
const ProtocolVersion uint16 = 0x0204

// Flag regroupe les drapeaux de l'en-tête de trame, envoyés en tête de chaque
// paquet pour décrire son encodage.
//
// FlagUnknownMask n'est pas un drapeau mais un masque : il couvre les bits
// qu'aMule ne sait pas interpréter. Si l'en-tête reçu en lève un, la trame est
// rejetée plutôt que devinée.
type Flag uint32

const (
	FlagZlib          Flag = 0x00000001
	FlagUTF8Numbers   Flag = 0x00000002
	FlagLargeTagCount Flag = 0x00000010
	FlagUnknownMask   Flag = 0xff7f7f08
)

// Opcode identifie l'opération portée par un paquet, requête comme réponse.
//
// La numérotation comporte des trous (0x0F, 0x12 à 0x15, 0x21, 0x24, 0x4C) :
// ce sont des opérations retirées au fil des versions, dont les codes n'ont pas
// été réattribués.
type Opcode uint8

const (
	OpNoop                     Opcode = 0x01
	OpAuthReq                  Opcode = 0x02
	OpAuthFail                 Opcode = 0x03
	OpAuthOk                   Opcode = 0x04
	OpFailed                   Opcode = 0x05
	OpStrings                  Opcode = 0x06
	OpMiscData                 Opcode = 0x07
	OpShutdown                 Opcode = 0x08
	OpAddLink                  Opcode = 0x09
	OpStatReq                  Opcode = 0x0A
	OpGetConnstate             Opcode = 0x0B
	OpStats                    Opcode = 0x0C
	OpGetDloadQueue            Opcode = 0x0D
	OpGetUloadQueue            Opcode = 0x0E
	OpGetSharedFiles           Opcode = 0x10
	OpSharedSetPrio            Opcode = 0x11
	OpPartfileSwapA4AFThis     Opcode = 0x16
	OpPartfileSwapA4AFThisAuto Opcode = 0x17
	OpPartfileSwapA4AFOthers   Opcode = 0x18
	OpPartfilePause            Opcode = 0x19
	OpPartfileResume           Opcode = 0x1A
	OpPartfileStop             Opcode = 0x1B
	OpPartfilePrioSet          Opcode = 0x1C
	OpPartfileDelete           Opcode = 0x1D
	OpPartfileSetCat           Opcode = 0x1E
	OpDloadQueue               Opcode = 0x1F
	OpUloadQueue               Opcode = 0x20
	OpSharedFiles              Opcode = 0x22
	OpSharedfilesReload        Opcode = 0x23
	OpRenameFile               Opcode = 0x25
	OpSearchStart              Opcode = 0x26
	OpSearchStop               Opcode = 0x27
	OpSearchResults            Opcode = 0x28
	OpSearchProgress           Opcode = 0x29
	OpDownloadSearchResult     Opcode = 0x2A
	OpIPFilterReload           Opcode = 0x2B
	OpGetServerList            Opcode = 0x2C
	OpServerList               Opcode = 0x2D
	OpServerDisconnect         Opcode = 0x2E
	OpServerConnect            Opcode = 0x2F
	OpServerRemove             Opcode = 0x30
	OpServerAdd                Opcode = 0x31
	OpServerUpdateFromURL      Opcode = 0x32
	OpAddlogline               Opcode = 0x33
	OpAdddebuglogline          Opcode = 0x34
	OpGetLog                   Opcode = 0x35
	OpGetDebuglog              Opcode = 0x36
	OpGetServerinfo            Opcode = 0x37
	OpLog                      Opcode = 0x38
	OpDebuglog                 Opcode = 0x39
	OpServerinfo               Opcode = 0x3A
	OpResetLog                 Opcode = 0x3B
	OpResetDebuglog            Opcode = 0x3C
	OpClearServerinfo          Opcode = 0x3D
	OpGetLastLogEntry          Opcode = 0x3E
	OpGetPreferences           Opcode = 0x3F
	OpSetPreferences           Opcode = 0x40
	OpCreateCategory           Opcode = 0x41
	OpUpdateCategory           Opcode = 0x42
	OpDeleteCategory           Opcode = 0x43
	OpGetStatsgraphs           Opcode = 0x44
	OpStatsgraphs              Opcode = 0x45
	OpGetStatstree             Opcode = 0x46
	OpStatstree                Opcode = 0x47
	OpKadStart                 Opcode = 0x48
	OpKadStop                  Opcode = 0x49
	OpConnect                  Opcode = 0x4A
	OpDisconnect               Opcode = 0x4B
	OpKadUpdateFromURL         Opcode = 0x4D
	OpKadBootstrapFromIP       Opcode = 0x4E
	OpAuthSalt                 Opcode = 0x4F
	OpAuthPasswd               Opcode = 0x50
	OpIPFilterUpdate           Opcode = 0x51
	OpGetUpdate                Opcode = 0x52
	OpClearCompleted           Opcode = 0x53
	OpClientSwapToAnotherFile  Opcode = 0x54
	OpSharedFileSetComment     Opcode = 0x55
	OpServerSetStaticPrio      Opcode = 0x56
	OpFriend                   Opcode = 0x57
)

// TagName identifie un champ à l'intérieur d'un paquet. Les tags sont
// hiérarchiques : un tag conteneur (par exemple TagPartfile) porte des
// sous-tags qui décrivent l'objet.
//
// L'octet de poids fort regroupe les tags par famille (0x03xx pour un fichier
// en cours de téléchargement, 0x05xx pour un serveur, 0x1xxx pour les
// préférences, etc.), ce qui laisse de larges intervalles inutilisés entre les
// familles.
type TagName uint16

const (
	TagString                        TagName = 0x0000
	TagPasswdHash                    TagName = 0x0001
	TagProtocolVersion               TagName = 0x0002
	TagVersionID                     TagName = 0x0003
	TagDetailLevel                   TagName = 0x0004
	TagConnstate                     TagName = 0x0005
	TagEd2kID                        TagName = 0x0006
	TagLogToStatus                   TagName = 0x0007
	TagBootstrapIP                   TagName = 0x0008
	TagBootstrapPort                 TagName = 0x0009
	TagClientID                      TagName = 0x000A
	TagPasswdSalt                    TagName = 0x000B
	TagCanZlib                       TagName = 0x000C
	TagCanUTF8Numbers                TagName = 0x000D
	TagCanNotify                     TagName = 0x000E
	TagECID                          TagName = 0x000F
	TagKadID                         TagName = 0x0010
	TagCanLargeTagCount              TagName = 0x0011
	TagCanPartialUpdate              TagName = 0x0012
	TagFileRemoved                   TagName = 0x0013
	TagPreferNoZlib                  TagName = 0x0014
	TagClientName                    TagName = 0x0100
	TagClientVersion                 TagName = 0x0101
	TagClientMod                     TagName = 0x0102
	TagStatsUlSpeed                  TagName = 0x0200
	TagStatsDlSpeed                  TagName = 0x0201
	TagStatsUlSpeedLimit             TagName = 0x0202
	TagStatsDlSpeedLimit             TagName = 0x0203
	TagStatsUpOverhead               TagName = 0x0204
	TagStatsDownOverhead             TagName = 0x0205
	TagStatsTotalSrcCount            TagName = 0x0206
	TagStatsBannedCount              TagName = 0x0207
	TagStatsUlQueueLen               TagName = 0x0208
	TagStatsEd2kUsers                TagName = 0x0209
	TagStatsKadUsers                 TagName = 0x020A
	TagStatsEd2kFiles                TagName = 0x020B
	TagStatsKadFiles                 TagName = 0x020C
	TagStatsLoggerMessage            TagName = 0x020D
	TagStatsKadFirewalledUDP         TagName = 0x020E
	TagStatsKadIndexedSources        TagName = 0x020F
	TagStatsKadIndexedKeywords       TagName = 0x0210
	TagStatsKadIndexedNotes          TagName = 0x0211
	TagStatsKadIndexedLoad           TagName = 0x0212
	TagStatsKadIPAddress             TagName = 0x0213
	TagStatsBuddyStatus              TagName = 0x0214
	TagStatsBuddyIP                  TagName = 0x0215
	TagStatsBuddyPort                TagName = 0x0216
	TagStatsKadInLanMode             TagName = 0x0217
	TagStatsTotalSentBytes           TagName = 0x0218
	TagStatsTotalReceivedBytes       TagName = 0x0219
	TagStatsSharedFileCount          TagName = 0x021A
	TagStatsKadNodes                 TagName = 0x021B
	TagPartfile                      TagName = 0x0300
	TagPartfileName                  TagName = 0x0301
	TagPartfilePartMetID             TagName = 0x0302
	TagPartfileSizeFull              TagName = 0x0303
	TagPartfileSizeXfer              TagName = 0x0304
	TagPartfileSizeXferUp            TagName = 0x0305
	TagPartfileSizeDone              TagName = 0x0306
	TagPartfileSpeed                 TagName = 0x0307
	TagPartfileStatus                TagName = 0x0308
	TagPartfilePrio                  TagName = 0x0309
	TagPartfileSourceCount           TagName = 0x030A
	TagPartfileSourceCountA4AF       TagName = 0x030B
	TagPartfileSourceCountNotCurrent TagName = 0x030C
	TagPartfileSourceCountXfer       TagName = 0x030D
	TagPartfileEd2kLink              TagName = 0x030E
	TagPartfileCat                   TagName = 0x030F
	TagPartfileLastRecv              TagName = 0x0310
	TagPartfileLastSeenComp          TagName = 0x0311
	TagPartfilePartStatus            TagName = 0x0312
	TagPartfileGapStatus             TagName = 0x0313
	TagPartfileReqStatus             TagName = 0x0314
	TagPartfileSourceNames           TagName = 0x0315
	TagPartfileComments              TagName = 0x0316
	TagPartfileStopped               TagName = 0x0317
	TagPartfileDownloadActive        TagName = 0x0318
	TagPartfileLostCorruption        TagName = 0x0319
	TagPartfileGainedCompression     TagName = 0x031A
	TagPartfileSavedICH              TagName = 0x031B
	TagPartfileSourceNamesCounts     TagName = 0x031C
	TagPartfileAvailableParts        TagName = 0x031D
	TagPartfileHash                  TagName = 0x031E
	TagPartfileShared                TagName = 0x031F
	TagPartfileHashedPartCount       TagName = 0x0320
	TagPartfileA4AFAuto              TagName = 0x0321
	TagPartfileA4AFSources           TagName = 0x0322
	TagKnownfile                     TagName = 0x0400
	TagKnownfileXferred              TagName = 0x0401
	TagKnownfileXferredAll           TagName = 0x0402
	TagKnownfileReqCount             TagName = 0x0403
	TagKnownfileReqCountAll          TagName = 0x0404
	TagKnownfileAcceptCount          TagName = 0x0405
	TagKnownfileAcceptCountAll       TagName = 0x0406
	TagKnownfileAICHMasterhash       TagName = 0x0407
	TagKnownfileFilename             TagName = 0x0408
	TagKnownfileCompleteSourcesLow   TagName = 0x0409
	TagKnownfileCompleteSourcesHigh  TagName = 0x040A
	TagKnownfilePrio                 TagName = 0x040B
	TagKnownfileOnQueue              TagName = 0x040C
	TagKnownfileCompleteSources      TagName = 0x040D
	TagKnownfileComment              TagName = 0x040E
	TagKnownfileRating               TagName = 0x040F
	TagServer                        TagName = 0x0500
	TagServerName                    TagName = 0x0501
	TagServerDesc                    TagName = 0x0502
	TagServerAddress                 TagName = 0x0503
	TagServerPing                    TagName = 0x0504
	TagServerUsers                   TagName = 0x0505
	TagServerUsersMax                TagName = 0x0506
	TagServerFiles                   TagName = 0x0507
	TagServerPrio                    TagName = 0x0508
	TagServerFailed                  TagName = 0x0509
	TagServerStatic                  TagName = 0x050A
	TagServerVersion                 TagName = 0x050B
	TagServerIP                      TagName = 0x050C
	TagServerPort                    TagName = 0x050D
	TagClient                        TagName = 0x0600
	TagClientSoftware                TagName = 0x0601
	TagClientScore                   TagName = 0x0602
	TagClientHash                    TagName = 0x0603
	TagClientFriendSlot              TagName = 0x0604
	TagClientWaitTime                TagName = 0x0605
	TagClientXferTime                TagName = 0x0606
	TagClientQueueTime               TagName = 0x0607
	TagClientLastTime                TagName = 0x0608
	TagClientUploadSession           TagName = 0x0609
	TagClientUploadTotal             TagName = 0x060A
	TagClientDownloadTotal           TagName = 0x060B
	TagClientDownloadState           TagName = 0x060C
	TagClientUpSpeed                 TagName = 0x060D
	TagClientDownSpeed               TagName = 0x060E
	TagClientFrom                    TagName = 0x060F
	TagClientUserIP                  TagName = 0x0610
	TagClientUserPort                TagName = 0x0611
	TagClientServerIP                TagName = 0x0612
	TagClientServerPort              TagName = 0x0613
	TagClientServerName              TagName = 0x0614
	TagClientSoftVerStr              TagName = 0x0615
	TagClientWaitingPosition         TagName = 0x0616
	TagClientIdentState              TagName = 0x0617
	TagClientObfuscationStatus       TagName = 0x0618
	TagClientCurrentlyunused1        TagName = 0x0619
	TagClientRemoteQueueRank         TagName = 0x061A
	TagClientDisableViewShared       TagName = 0x061B
	TagClientUploadState             TagName = 0x061C
	TagClientExtProtocol             TagName = 0x061D
	TagClientUserID                  TagName = 0x061E
	TagClientUploadFile              TagName = 0x061F
	TagClientRequestFile             TagName = 0x0620
	TagClientA4AFFiles               TagName = 0x0621
	TagClientOldRemoteQueueRank      TagName = 0x0622
	TagClientKadPort                 TagName = 0x0623
	TagClientPartStatus              TagName = 0x0624
	TagClientNextRequestedPart       TagName = 0x0625
	TagClientLastDownloadingPart     TagName = 0x0626
	TagClientRemoteFilename          TagName = 0x0627
	TagClientModVersion              TagName = 0x0628
	TagClientOSInfo                  TagName = 0x0629
	TagClientAvailableParts          TagName = 0x062A
	TagClientUploadPartStatus        TagName = 0x062B
	TagSearchfile                    TagName = 0x0700
	TagSearchType                    TagName = 0x0701
	TagSearchName                    TagName = 0x0702
	TagSearchMinSize                 TagName = 0x0703
	TagSearchMaxSize                 TagName = 0x0704
	TagSearchFileType                TagName = 0x0705
	TagSearchExtension               TagName = 0x0706
	TagSearchAvailability            TagName = 0x0707
	TagSearchStatus                  TagName = 0x0708
	TagSearchParent                  TagName = 0x0709
	TagFriend                        TagName = 0x0800
	TagFriendName                    TagName = 0x0801
	TagFriendHash                    TagName = 0x0802
	TagFriendIP                      TagName = 0x0803
	TagFriendPort                    TagName = 0x0804
	TagFriendClient                  TagName = 0x0805
	TagFriendAdd                     TagName = 0x0806
	TagFriendRemove                  TagName = 0x0807
	TagFriendFriendslot              TagName = 0x0808
	TagFriendShared                  TagName = 0x0809
	TagSelectPrefs                   TagName = 0x1000
	TagPrefsCategories               TagName = 0x1100
	TagCategory                      TagName = 0x1101
	TagCategoryTitle                 TagName = 0x1102
	TagCategoryPath                  TagName = 0x1103
	TagCategoryComment               TagName = 0x1104
	TagCategoryColor                 TagName = 0x1105
	TagCategoryPrio                  TagName = 0x1106
	TagPrefsGeneral                  TagName = 0x1200
	TagUserNick                      TagName = 0x1201
	TagUserHash                      TagName = 0x1202
	TagUserHost                      TagName = 0x1203
	TagGeneralCheckNewVersion        TagName = 0x1204
	TagPrefsConnections              TagName = 0x1300
	TagConnDlCap                     TagName = 0x1301
	TagConnUlCap                     TagName = 0x1302
	TagConnMaxDl                     TagName = 0x1303
	TagConnMaxUl                     TagName = 0x1304
	TagConnSlotAllocation            TagName = 0x1305
	TagConnTCPPort                   TagName = 0x1306
	TagConnUDPPort                   TagName = 0x1307
	TagConnUDPDisable                TagName = 0x1308
	TagConnMaxFileSources            TagName = 0x1309
	TagConnMaxConn                   TagName = 0x130A
	TagConnAutoconnect               TagName = 0x130B
	TagConnReconnect                 TagName = 0x130C
	TagNetworkEd2k                   TagName = 0x130D
	TagNetworkKademlia               TagName = 0x130E
	TagPrefsMessagefilter            TagName = 0x1400
	TagMsgfilterEnabled              TagName = 0x1401
	TagMsgfilterAll                  TagName = 0x1402
	TagMsgfilterFriends              TagName = 0x1403
	TagMsgfilterSecure               TagName = 0x1404
	TagMsgfilterByKeyword            TagName = 0x1405
	TagMsgfilterKeywords             TagName = 0x1406
	TagPrefsRemotectrl               TagName = 0x1500
	TagWebserverAutorun              TagName = 0x1501
	TagWebserverPort                 TagName = 0x1502
	TagWebserverGuest                TagName = 0x1503
	TagWebserverUseGzip              TagName = 0x1504
	TagWebserverRefresh              TagName = 0x1505
	TagWebserverTemplate             TagName = 0x1506
	TagPrefsOnlinesig                TagName = 0x1600
	TagOnlinesigEnabled              TagName = 0x1601
	TagPrefsServers                  TagName = 0x1700
	TagServersRemoveDead             TagName = 0x1701
	TagServersDeadServerRetries      TagName = 0x1702
	TagServersAutoUpdate             TagName = 0x1703
	TagServersURLList                TagName = 0x1704
	TagServersAddFromServer          TagName = 0x1705
	TagServersAddFromClient          TagName = 0x1706
	TagServersUseScoreSystem         TagName = 0x1707
	TagServersSmartIDCheck           TagName = 0x1708
	TagServersSafeServerConnect      TagName = 0x1709
	TagServersAutoconnStaticOnly     TagName = 0x170A
	TagServersManualHighPrio         TagName = 0x170B
	TagServersUpdateURL              TagName = 0x170C
	TagPrefsFiles                    TagName = 0x1800
	TagFilesICHEnabled               TagName = 0x1801
	TagFilesAICHTrust                TagName = 0x1802
	TagFilesNewPaused                TagName = 0x1803
	TagFilesNewAutoDlPrio            TagName = 0x1804
	TagFilesPreviewPrio              TagName = 0x1805
	TagFilesNewAutoUlPrio            TagName = 0x1806
	TagFilesUlFullChunks             TagName = 0x1807
	TagFilesStartNextPaused          TagName = 0x1808
	TagFilesResumeSameCat            TagName = 0x1809
	TagFilesSaveSources              TagName = 0x180A
	TagFilesExtractMetadata          TagName = 0x180B
	TagFilesAllocFullSize            TagName = 0x180C
	TagFilesCheckFreeSpace           TagName = 0x180D
	TagFilesMinFreeSpace             TagName = 0x180E
	TagFilesCreateNormal             TagName = 0x180F
	TagPrefsDirectories              TagName = 0x1A00
	TagDirectoriesIncoming           TagName = 0x1A01
	TagDirectoriesTemp               TagName = 0x1A02
	TagDirectoriesShared             TagName = 0x1A03
	TagDirectoriesShareHidden        TagName = 0x1A04
	TagDirectoriesAutoRescan         TagName = 0x1A05
	TagPrefsStatistics               TagName = 0x1B00
	TagStatsgraphWidth               TagName = 0x1B01
	TagStatsgraphScale               TagName = 0x1B02
	TagStatsgraphLast                TagName = 0x1B03
	TagStatsgraphData                TagName = 0x1B04
	TagStattreeCapping               TagName = 0x1B05
	TagStattreeNode                  TagName = 0x1B06
	TagStatNodeValue                 TagName = 0x1B07
	TagStatValueType                 TagName = 0x1B08
	TagStattreeNodeID                TagName = 0x1B09
	TagStatsgraphDataConn            TagName = 0x1B0A
	TagStatsgraphSessionDl           TagName = 0x1B0B
	TagStatsgraphSessionUl           TagName = 0x1B0C
	TagStatsgraphSessionKad          TagName = 0x1B0D
	TagStatsgraphSessionTimespan     TagName = 0x1B0E
	TagPrefsSecurity                 TagName = 0x1C00
	TagSecurityCanSeeShares          TagName = 0x1C01
	TagIPFilterClients               TagName = 0x1C02
	TagIPFilterServers               TagName = 0x1C03
	TagIPFilterAutoUpdate            TagName = 0x1C04
	TagIPFilterUpdateURL             TagName = 0x1C05
	TagIPFilterLevel                 TagName = 0x1C06
	TagIPFilterFilterLan             TagName = 0x1C07
	TagSecurityUseSecident           TagName = 0x1C08
	TagSecurityObfuscationSupported  TagName = 0x1C09
	TagSecurityObfuscationRequested  TagName = 0x1C0A
	TagSecurityObfuscationRequired   TagName = 0x1C0B
	TagPrefsCoretweaks               TagName = 0x1D00
	TagCoretwMaxConnPerFive          TagName = 0x1D01
	TagCoretwVerbose                 TagName = 0x1D02
	TagCoretwFilebuffer              TagName = 0x1D03
	TagCoretwUlQueue                 TagName = 0x1D04
	TagCoretwSrvKeepaliveTimeout     TagName = 0x1D05
	TagPrefsKademlia                 TagName = 0x1E00
	TagKademliaUpdateURL             TagName = 0x1E01
)

// TagType décrit comment lire la valeur d'un tag : c'est cet octet, et non le
// nom du tag, qui détermine le décodage de la charge utile.
type TagType uint8

const (
	TypeUnknown TagType = 0
	TypeCustom  TagType = 1
	TypeUint8   TagType = 2
	TypeUint16  TagType = 3
	TypeUint32  TagType = 4
	TypeUint64  TagType = 5
	TypeString  TagType = 6
	TypeDouble  TagType = 7
	TypeIPv4    TagType = 8
	TypeHash16  TagType = 9
	TypeUint128 TagType = 10
)

// DetailLevel indique au démon la quantité de détail attendue dans sa réponse.
// Il se transmet via TagDetailLevel dans la requête.
type DetailLevel uint8

const (
	DetailCmd       DetailLevel = 0x00
	DetailWeb       DetailLevel = 0x01
	DetailFull      DetailLevel = 0x02
	DetailUpdate    DetailLevel = 0x03
	DetailIncUpdate DetailLevel = 0x04
)

// SearchType désigne le réseau sur lequel lancer une recherche.
type SearchType uint8

const (
	SearchLocal  SearchType = 0x00
	SearchGlobal SearchType = 0x01
	SearchKad    SearchType = 0x02
	SearchWeb    SearchType = 0x03
)

// StatValueType décrit l'unité d'un nœud de l'arbre de statistiques, pour le
// mettre en forme correctement (octets, durée, débit, etc.).
type StatValueType uint8

const (
	StatValueInteger StatValueType = 0x00
	StatValueIstring StatValueType = 0x01
	StatValueBytes   StatValueType = 0x02
	StatValueIshort  StatValueType = 0x03
	StatValueTime    StatValueType = 0x04
	StatValueSpeed   StatValueType = 0x05
	StatValueString  StatValueType = 0x06
	StatValueDouble  StatValueType = 0x07
)

// PrefsSelection est un masque de bits qui sélectionne les sections de
// préférences à demander ou à modifier. Les valeurs 0x00000100 et 0x00004000 et
// au-delà ne sont pas attribuées ; l'absence de 0x00000100 correspond à une
// section retirée.
type PrefsSelection uint32

const (
	PrefsCategories     PrefsSelection = 0x00000001
	PrefsGeneral        PrefsSelection = 0x00000002
	PrefsConnections    PrefsSelection = 0x00000004
	PrefsMessagefilter  PrefsSelection = 0x00000008
	PrefsRemotecontrols PrefsSelection = 0x00000010
	PrefsOnlinesig      PrefsSelection = 0x00000020
	PrefsServers        PrefsSelection = 0x00000040
	PrefsFiles          PrefsSelection = 0x00000080
	PrefsDirectories    PrefsSelection = 0x00000200
	PrefsStatistics     PrefsSelection = 0x00000400
	PrefsSecurity       PrefsSelection = 0x00000800
	PrefsCoretweaks     PrefsSelection = 0x00001000
	PrefsKademlia       PrefsSelection = 0x00002000
)

// opcodeNames et tagNames servent uniquement au diagnostic : un journal qui
// nomme le code inattendu qu'il a reçu épargne une longue recherche.
// Le linter voit « Passwd » dans les libellés et y soupçonne un secret : ce ne
// sont que les noms symboliques des opcodes, écrits pour les journaux.
var opcodeNames = map[Opcode]string{ //nolint:gosec // libellés de diagnostic, aucun secret
	OpNoop:                     "OpNoop",
	OpAuthReq:                  "OpAuthReq",
	OpAuthFail:                 "OpAuthFail",
	OpAuthOk:                   "OpAuthOk",
	OpFailed:                   "OpFailed",
	OpStrings:                  "OpStrings",
	OpMiscData:                 "OpMiscData",
	OpShutdown:                 "OpShutdown",
	OpAddLink:                  "OpAddLink",
	OpStatReq:                  "OpStatReq",
	OpGetConnstate:             "OpGetConnstate",
	OpStats:                    "OpStats",
	OpGetDloadQueue:            "OpGetDloadQueue",
	OpGetUloadQueue:            "OpGetUloadQueue",
	OpGetSharedFiles:           "OpGetSharedFiles",
	OpSharedSetPrio:            "OpSharedSetPrio",
	OpPartfileSwapA4AFThis:     "OpPartfileSwapA4AFThis",
	OpPartfileSwapA4AFThisAuto: "OpPartfileSwapA4AFThisAuto",
	OpPartfileSwapA4AFOthers:   "OpPartfileSwapA4AFOthers",
	OpPartfilePause:            "OpPartfilePause",
	OpPartfileResume:           "OpPartfileResume",
	OpPartfileStop:             "OpPartfileStop",
	OpPartfilePrioSet:          "OpPartfilePrioSet",
	OpPartfileDelete:           "OpPartfileDelete",
	OpPartfileSetCat:           "OpPartfileSetCat",
	OpDloadQueue:               "OpDloadQueue",
	OpUloadQueue:               "OpUloadQueue",
	OpSharedFiles:              "OpSharedFiles",
	OpSharedfilesReload:        "OpSharedfilesReload",
	OpRenameFile:               "OpRenameFile",
	OpSearchStart:              "OpSearchStart",
	OpSearchStop:               "OpSearchStop",
	OpSearchResults:            "OpSearchResults",
	OpSearchProgress:           "OpSearchProgress",
	OpDownloadSearchResult:     "OpDownloadSearchResult",
	OpIPFilterReload:           "OpIPFilterReload",
	OpGetServerList:            "OpGetServerList",
	OpServerList:               "OpServerList",
	OpServerDisconnect:         "OpServerDisconnect",
	OpServerConnect:            "OpServerConnect",
	OpServerRemove:             "OpServerRemove",
	OpServerAdd:                "OpServerAdd",
	OpServerUpdateFromURL:      "OpServerUpdateFromURL",
	OpAddlogline:               "OpAddlogline",
	OpAdddebuglogline:          "OpAdddebuglogline",
	OpGetLog:                   "OpGetLog",
	OpGetDebuglog:              "OpGetDebuglog",
	OpGetServerinfo:            "OpGetServerinfo",
	OpLog:                      "OpLog",
	OpDebuglog:                 "OpDebuglog",
	OpServerinfo:               "OpServerinfo",
	OpResetLog:                 "OpResetLog",
	OpResetDebuglog:            "OpResetDebuglog",
	OpClearServerinfo:          "OpClearServerinfo",
	OpGetLastLogEntry:          "OpGetLastLogEntry",
	OpGetPreferences:           "OpGetPreferences",
	OpSetPreferences:           "OpSetPreferences",
	OpCreateCategory:           "OpCreateCategory",
	OpUpdateCategory:           "OpUpdateCategory",
	OpDeleteCategory:           "OpDeleteCategory",
	OpGetStatsgraphs:           "OpGetStatsgraphs",
	OpStatsgraphs:              "OpStatsgraphs",
	OpGetStatstree:             "OpGetStatstree",
	OpStatstree:                "OpStatstree",
	OpKadStart:                 "OpKadStart",
	OpKadStop:                  "OpKadStop",
	OpConnect:                  "OpConnect",
	OpDisconnect:               "OpDisconnect",
	OpKadUpdateFromURL:         "OpKadUpdateFromURL",
	OpKadBootstrapFromIP:       "OpKadBootstrapFromIP",
	OpAuthSalt:                 "OpAuthSalt",
	OpAuthPasswd:               "OpAuthPasswd",
	OpIPFilterUpdate:           "OpIPFilterUpdate",
	OpGetUpdate:                "OpGetUpdate",
	OpClearCompleted:           "OpClearCompleted",
	OpClientSwapToAnotherFile:  "OpClientSwapToAnotherFile",
	OpSharedFileSetComment:     "OpSharedFileSetComment",
	OpServerSetStaticPrio:      "OpServerSetStaticPrio",
	OpFriend:                   "OpFriend",
}

var tagNames = map[TagName]string{
	TagString:                        "TagString",
	TagPasswdHash:                    "TagPasswdHash",
	TagProtocolVersion:               "TagProtocolVersion",
	TagVersionID:                     "TagVersionID",
	TagDetailLevel:                   "TagDetailLevel",
	TagConnstate:                     "TagConnstate",
	TagEd2kID:                        "TagEd2kID",
	TagLogToStatus:                   "TagLogToStatus",
	TagBootstrapIP:                   "TagBootstrapIP",
	TagBootstrapPort:                 "TagBootstrapPort",
	TagClientID:                      "TagClientID",
	TagPasswdSalt:                    "TagPasswdSalt",
	TagCanZlib:                       "TagCanZlib",
	TagCanUTF8Numbers:                "TagCanUTF8Numbers",
	TagCanNotify:                     "TagCanNotify",
	TagECID:                          "TagECID",
	TagKadID:                         "TagKadID",
	TagCanLargeTagCount:              "TagCanLargeTagCount",
	TagCanPartialUpdate:              "TagCanPartialUpdate",
	TagFileRemoved:                   "TagFileRemoved",
	TagPreferNoZlib:                  "TagPreferNoZlib",
	TagClientName:                    "TagClientName",
	TagClientVersion:                 "TagClientVersion",
	TagClientMod:                     "TagClientMod",
	TagStatsUlSpeed:                  "TagStatsUlSpeed",
	TagStatsDlSpeed:                  "TagStatsDlSpeed",
	TagStatsUlSpeedLimit:             "TagStatsUlSpeedLimit",
	TagStatsDlSpeedLimit:             "TagStatsDlSpeedLimit",
	TagStatsUpOverhead:               "TagStatsUpOverhead",
	TagStatsDownOverhead:             "TagStatsDownOverhead",
	TagStatsTotalSrcCount:            "TagStatsTotalSrcCount",
	TagStatsBannedCount:              "TagStatsBannedCount",
	TagStatsUlQueueLen:               "TagStatsUlQueueLen",
	TagStatsEd2kUsers:                "TagStatsEd2kUsers",
	TagStatsKadUsers:                 "TagStatsKadUsers",
	TagStatsEd2kFiles:                "TagStatsEd2kFiles",
	TagStatsKadFiles:                 "TagStatsKadFiles",
	TagStatsLoggerMessage:            "TagStatsLoggerMessage",
	TagStatsKadFirewalledUDP:         "TagStatsKadFirewalledUDP",
	TagStatsKadIndexedSources:        "TagStatsKadIndexedSources",
	TagStatsKadIndexedKeywords:       "TagStatsKadIndexedKeywords",
	TagStatsKadIndexedNotes:          "TagStatsKadIndexedNotes",
	TagStatsKadIndexedLoad:           "TagStatsKadIndexedLoad",
	TagStatsKadIPAddress:             "TagStatsKadIPAddress",
	TagStatsBuddyStatus:              "TagStatsBuddyStatus",
	TagStatsBuddyIP:                  "TagStatsBuddyIP",
	TagStatsBuddyPort:                "TagStatsBuddyPort",
	TagStatsKadInLanMode:             "TagStatsKadInLanMode",
	TagStatsTotalSentBytes:           "TagStatsTotalSentBytes",
	TagStatsTotalReceivedBytes:       "TagStatsTotalReceivedBytes",
	TagStatsSharedFileCount:          "TagStatsSharedFileCount",
	TagStatsKadNodes:                 "TagStatsKadNodes",
	TagPartfile:                      "TagPartfile",
	TagPartfileName:                  "TagPartfileName",
	TagPartfilePartMetID:             "TagPartfilePartMetID",
	TagPartfileSizeFull:              "TagPartfileSizeFull",
	TagPartfileSizeXfer:              "TagPartfileSizeXfer",
	TagPartfileSizeXferUp:            "TagPartfileSizeXferUp",
	TagPartfileSizeDone:              "TagPartfileSizeDone",
	TagPartfileSpeed:                 "TagPartfileSpeed",
	TagPartfileStatus:                "TagPartfileStatus",
	TagPartfilePrio:                  "TagPartfilePrio",
	TagPartfileSourceCount:           "TagPartfileSourceCount",
	TagPartfileSourceCountA4AF:       "TagPartfileSourceCountA4AF",
	TagPartfileSourceCountNotCurrent: "TagPartfileSourceCountNotCurrent",
	TagPartfileSourceCountXfer:       "TagPartfileSourceCountXfer",
	TagPartfileEd2kLink:              "TagPartfileEd2kLink",
	TagPartfileCat:                   "TagPartfileCat",
	TagPartfileLastRecv:              "TagPartfileLastRecv",
	TagPartfileLastSeenComp:          "TagPartfileLastSeenComp",
	TagPartfilePartStatus:            "TagPartfilePartStatus",
	TagPartfileGapStatus:             "TagPartfileGapStatus",
	TagPartfileReqStatus:             "TagPartfileReqStatus",
	TagPartfileSourceNames:           "TagPartfileSourceNames",
	TagPartfileComments:              "TagPartfileComments",
	TagPartfileStopped:               "TagPartfileStopped",
	TagPartfileDownloadActive:        "TagPartfileDownloadActive",
	TagPartfileLostCorruption:        "TagPartfileLostCorruption",
	TagPartfileGainedCompression:     "TagPartfileGainedCompression",
	TagPartfileSavedICH:              "TagPartfileSavedICH",
	TagPartfileSourceNamesCounts:     "TagPartfileSourceNamesCounts",
	TagPartfileAvailableParts:        "TagPartfileAvailableParts",
	TagPartfileHash:                  "TagPartfileHash",
	TagPartfileShared:                "TagPartfileShared",
	TagPartfileHashedPartCount:       "TagPartfileHashedPartCount",
	TagPartfileA4AFAuto:              "TagPartfileA4AFAuto",
	TagPartfileA4AFSources:           "TagPartfileA4AFSources",
	TagKnownfile:                     "TagKnownfile",
	TagKnownfileXferred:              "TagKnownfileXferred",
	TagKnownfileXferredAll:           "TagKnownfileXferredAll",
	TagKnownfileReqCount:             "TagKnownfileReqCount",
	TagKnownfileReqCountAll:          "TagKnownfileReqCountAll",
	TagKnownfileAcceptCount:          "TagKnownfileAcceptCount",
	TagKnownfileAcceptCountAll:       "TagKnownfileAcceptCountAll",
	TagKnownfileAICHMasterhash:       "TagKnownfileAICHMasterhash",
	TagKnownfileFilename:             "TagKnownfileFilename",
	TagKnownfileCompleteSourcesLow:   "TagKnownfileCompleteSourcesLow",
	TagKnownfileCompleteSourcesHigh:  "TagKnownfileCompleteSourcesHigh",
	TagKnownfilePrio:                 "TagKnownfilePrio",
	TagKnownfileOnQueue:              "TagKnownfileOnQueue",
	TagKnownfileCompleteSources:      "TagKnownfileCompleteSources",
	TagKnownfileComment:              "TagKnownfileComment",
	TagKnownfileRating:               "TagKnownfileRating",
	TagServer:                        "TagServer",
	TagServerName:                    "TagServerName",
	TagServerDesc:                    "TagServerDesc",
	TagServerAddress:                 "TagServerAddress",
	TagServerPing:                    "TagServerPing",
	TagServerUsers:                   "TagServerUsers",
	TagServerUsersMax:                "TagServerUsersMax",
	TagServerFiles:                   "TagServerFiles",
	TagServerPrio:                    "TagServerPrio",
	TagServerFailed:                  "TagServerFailed",
	TagServerStatic:                  "TagServerStatic",
	TagServerVersion:                 "TagServerVersion",
	TagServerIP:                      "TagServerIP",
	TagServerPort:                    "TagServerPort",
	TagClient:                        "TagClient",
	TagClientSoftware:                "TagClientSoftware",
	TagClientScore:                   "TagClientScore",
	TagClientHash:                    "TagClientHash",
	TagClientFriendSlot:              "TagClientFriendSlot",
	TagClientWaitTime:                "TagClientWaitTime",
	TagClientXferTime:                "TagClientXferTime",
	TagClientQueueTime:               "TagClientQueueTime",
	TagClientLastTime:                "TagClientLastTime",
	TagClientUploadSession:           "TagClientUploadSession",
	TagClientUploadTotal:             "TagClientUploadTotal",
	TagClientDownloadTotal:           "TagClientDownloadTotal",
	TagClientDownloadState:           "TagClientDownloadState",
	TagClientUpSpeed:                 "TagClientUpSpeed",
	TagClientDownSpeed:               "TagClientDownSpeed",
	TagClientFrom:                    "TagClientFrom",
	TagClientUserIP:                  "TagClientUserIP",
	TagClientUserPort:                "TagClientUserPort",
	TagClientServerIP:                "TagClientServerIP",
	TagClientServerPort:              "TagClientServerPort",
	TagClientServerName:              "TagClientServerName",
	TagClientSoftVerStr:              "TagClientSoftVerStr",
	TagClientWaitingPosition:         "TagClientWaitingPosition",
	TagClientIdentState:              "TagClientIdentState",
	TagClientObfuscationStatus:       "TagClientObfuscationStatus",
	TagClientCurrentlyunused1:        "TagClientCurrentlyunused1",
	TagClientRemoteQueueRank:         "TagClientRemoteQueueRank",
	TagClientDisableViewShared:       "TagClientDisableViewShared",
	TagClientUploadState:             "TagClientUploadState",
	TagClientExtProtocol:             "TagClientExtProtocol",
	TagClientUserID:                  "TagClientUserID",
	TagClientUploadFile:              "TagClientUploadFile",
	TagClientRequestFile:             "TagClientRequestFile",
	TagClientA4AFFiles:               "TagClientA4AFFiles",
	TagClientOldRemoteQueueRank:      "TagClientOldRemoteQueueRank",
	TagClientKadPort:                 "TagClientKadPort",
	TagClientPartStatus:              "TagClientPartStatus",
	TagClientNextRequestedPart:       "TagClientNextRequestedPart",
	TagClientLastDownloadingPart:     "TagClientLastDownloadingPart",
	TagClientRemoteFilename:          "TagClientRemoteFilename",
	TagClientModVersion:              "TagClientModVersion",
	TagClientOSInfo:                  "TagClientOSInfo",
	TagClientAvailableParts:          "TagClientAvailableParts",
	TagClientUploadPartStatus:        "TagClientUploadPartStatus",
	TagSearchfile:                    "TagSearchfile",
	TagSearchType:                    "TagSearchType",
	TagSearchName:                    "TagSearchName",
	TagSearchMinSize:                 "TagSearchMinSize",
	TagSearchMaxSize:                 "TagSearchMaxSize",
	TagSearchFileType:                "TagSearchFileType",
	TagSearchExtension:               "TagSearchExtension",
	TagSearchAvailability:            "TagSearchAvailability",
	TagSearchStatus:                  "TagSearchStatus",
	TagSearchParent:                  "TagSearchParent",
	TagFriend:                        "TagFriend",
	TagFriendName:                    "TagFriendName",
	TagFriendHash:                    "TagFriendHash",
	TagFriendIP:                      "TagFriendIP",
	TagFriendPort:                    "TagFriendPort",
	TagFriendClient:                  "TagFriendClient",
	TagFriendAdd:                     "TagFriendAdd",
	TagFriendRemove:                  "TagFriendRemove",
	TagFriendFriendslot:              "TagFriendFriendslot",
	TagFriendShared:                  "TagFriendShared",
	TagSelectPrefs:                   "TagSelectPrefs",
	TagPrefsCategories:               "TagPrefsCategories",
	TagCategory:                      "TagCategory",
	TagCategoryTitle:                 "TagCategoryTitle",
	TagCategoryPath:                  "TagCategoryPath",
	TagCategoryComment:               "TagCategoryComment",
	TagCategoryColor:                 "TagCategoryColor",
	TagCategoryPrio:                  "TagCategoryPrio",
	TagPrefsGeneral:                  "TagPrefsGeneral",
	TagUserNick:                      "TagUserNick",
	TagUserHash:                      "TagUserHash",
	TagUserHost:                      "TagUserHost",
	TagGeneralCheckNewVersion:        "TagGeneralCheckNewVersion",
	TagPrefsConnections:              "TagPrefsConnections",
	TagConnDlCap:                     "TagConnDlCap",
	TagConnUlCap:                     "TagConnUlCap",
	TagConnMaxDl:                     "TagConnMaxDl",
	TagConnMaxUl:                     "TagConnMaxUl",
	TagConnSlotAllocation:            "TagConnSlotAllocation",
	TagConnTCPPort:                   "TagConnTCPPort",
	TagConnUDPPort:                   "TagConnUDPPort",
	TagConnUDPDisable:                "TagConnUDPDisable",
	TagConnMaxFileSources:            "TagConnMaxFileSources",
	TagConnMaxConn:                   "TagConnMaxConn",
	TagConnAutoconnect:               "TagConnAutoconnect",
	TagConnReconnect:                 "TagConnReconnect",
	TagNetworkEd2k:                   "TagNetworkEd2k",
	TagNetworkKademlia:               "TagNetworkKademlia",
	TagPrefsMessagefilter:            "TagPrefsMessagefilter",
	TagMsgfilterEnabled:              "TagMsgfilterEnabled",
	TagMsgfilterAll:                  "TagMsgfilterAll",
	TagMsgfilterFriends:              "TagMsgfilterFriends",
	TagMsgfilterSecure:               "TagMsgfilterSecure",
	TagMsgfilterByKeyword:            "TagMsgfilterByKeyword",
	TagMsgfilterKeywords:             "TagMsgfilterKeywords",
	TagPrefsRemotectrl:               "TagPrefsRemotectrl",
	TagWebserverAutorun:              "TagWebserverAutorun",
	TagWebserverPort:                 "TagWebserverPort",
	TagWebserverGuest:                "TagWebserverGuest",
	TagWebserverUseGzip:              "TagWebserverUseGzip",
	TagWebserverRefresh:              "TagWebserverRefresh",
	TagWebserverTemplate:             "TagWebserverTemplate",
	TagPrefsOnlinesig:                "TagPrefsOnlinesig",
	TagOnlinesigEnabled:              "TagOnlinesigEnabled",
	TagPrefsServers:                  "TagPrefsServers",
	TagServersRemoveDead:             "TagServersRemoveDead",
	TagServersDeadServerRetries:      "TagServersDeadServerRetries",
	TagServersAutoUpdate:             "TagServersAutoUpdate",
	TagServersURLList:                "TagServersURLList",
	TagServersAddFromServer:          "TagServersAddFromServer",
	TagServersAddFromClient:          "TagServersAddFromClient",
	TagServersUseScoreSystem:         "TagServersUseScoreSystem",
	TagServersSmartIDCheck:           "TagServersSmartIDCheck",
	TagServersSafeServerConnect:      "TagServersSafeServerConnect",
	TagServersAutoconnStaticOnly:     "TagServersAutoconnStaticOnly",
	TagServersManualHighPrio:         "TagServersManualHighPrio",
	TagServersUpdateURL:              "TagServersUpdateURL",
	TagPrefsFiles:                    "TagPrefsFiles",
	TagFilesICHEnabled:               "TagFilesICHEnabled",
	TagFilesAICHTrust:                "TagFilesAICHTrust",
	TagFilesNewPaused:                "TagFilesNewPaused",
	TagFilesNewAutoDlPrio:            "TagFilesNewAutoDlPrio",
	TagFilesPreviewPrio:              "TagFilesPreviewPrio",
	TagFilesNewAutoUlPrio:            "TagFilesNewAutoUlPrio",
	TagFilesUlFullChunks:             "TagFilesUlFullChunks",
	TagFilesStartNextPaused:          "TagFilesStartNextPaused",
	TagFilesResumeSameCat:            "TagFilesResumeSameCat",
	TagFilesSaveSources:              "TagFilesSaveSources",
	TagFilesExtractMetadata:          "TagFilesExtractMetadata",
	TagFilesAllocFullSize:            "TagFilesAllocFullSize",
	TagFilesCheckFreeSpace:           "TagFilesCheckFreeSpace",
	TagFilesMinFreeSpace:             "TagFilesMinFreeSpace",
	TagFilesCreateNormal:             "TagFilesCreateNormal",
	TagPrefsDirectories:              "TagPrefsDirectories",
	TagDirectoriesIncoming:           "TagDirectoriesIncoming",
	TagDirectoriesTemp:               "TagDirectoriesTemp",
	TagDirectoriesShared:             "TagDirectoriesShared",
	TagDirectoriesShareHidden:        "TagDirectoriesShareHidden",
	TagDirectoriesAutoRescan:         "TagDirectoriesAutoRescan",
	TagPrefsStatistics:               "TagPrefsStatistics",
	TagStatsgraphWidth:               "TagStatsgraphWidth",
	TagStatsgraphScale:               "TagStatsgraphScale",
	TagStatsgraphLast:                "TagStatsgraphLast",
	TagStatsgraphData:                "TagStatsgraphData",
	TagStattreeCapping:               "TagStattreeCapping",
	TagStattreeNode:                  "TagStattreeNode",
	TagStatNodeValue:                 "TagStatNodeValue",
	TagStatValueType:                 "TagStatValueType",
	TagStattreeNodeID:                "TagStattreeNodeID",
	TagStatsgraphDataConn:            "TagStatsgraphDataConn",
	TagStatsgraphSessionDl:           "TagStatsgraphSessionDl",
	TagStatsgraphSessionUl:           "TagStatsgraphSessionUl",
	TagStatsgraphSessionKad:          "TagStatsgraphSessionKad",
	TagStatsgraphSessionTimespan:     "TagStatsgraphSessionTimespan",
	TagPrefsSecurity:                 "TagPrefsSecurity",
	TagSecurityCanSeeShares:          "TagSecurityCanSeeShares",
	TagIPFilterClients:               "TagIPFilterClients",
	TagIPFilterServers:               "TagIPFilterServers",
	TagIPFilterAutoUpdate:            "TagIPFilterAutoUpdate",
	TagIPFilterUpdateURL:             "TagIPFilterUpdateURL",
	TagIPFilterLevel:                 "TagIPFilterLevel",
	TagIPFilterFilterLan:             "TagIPFilterFilterLan",
	TagSecurityUseSecident:           "TagSecurityUseSecident",
	TagSecurityObfuscationSupported:  "TagSecurityObfuscationSupported",
	TagSecurityObfuscationRequested:  "TagSecurityObfuscationRequested",
	TagSecurityObfuscationRequired:   "TagSecurityObfuscationRequired",
	TagPrefsCoretweaks:               "TagPrefsCoretweaks",
	TagCoretwMaxConnPerFive:          "TagCoretwMaxConnPerFive",
	TagCoretwVerbose:                 "TagCoretwVerbose",
	TagCoretwFilebuffer:              "TagCoretwFilebuffer",
	TagCoretwUlQueue:                 "TagCoretwUlQueue",
	TagCoretwSrvKeepaliveTimeout:     "TagCoretwSrvKeepaliveTimeout",
	TagPrefsKademlia:                 "TagPrefsKademlia",
	TagKademliaUpdateURL:             "TagKademliaUpdateURL",
}

// String rend le nom symbolique de l'opcode, ou sa valeur hexadécimale s'il est
// inconnu de cette table.
func (o Opcode) String() string {
	if n, ok := opcodeNames[o]; ok {
		return n
	}
	return fmt.Sprintf("0x%02x", uint8(o))
}

// String rend le nom symbolique du tag, ou sa valeur hexadécimale s'il est
// inconnu de cette table.
func (t TagName) String() string {
	if n, ok := tagNames[t]; ok {
		return n
	}
	return fmt.Sprintf("0x%04x", uint16(t))
}
