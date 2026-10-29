package devseed

// mvpCommerceCategory is a stable taxonomy key stored in raw_attributes.
type mvpCommerceCategory string

const (
	catKitchenStorage   mvpCommerceCategory = "kitchen_storage"
	catHomeTextile      mvpCommerceCategory = "home_textile"
	catCleaning         mvpCommerceCategory = "cleaning"
	catBathroom         mvpCommerceCategory = "bathroom"
	catLightDecor       mvpCommerceCategory = "light_decor"
	catHomeOrganization mvpCommerceCategory = "home_organization"
)

// mvpCommerceSegment is used for demand/stock shaping and raw_attributes.segment.
type mvpCommerceSegment string

const (
	segLeaders    mvpCommerceSegment = "leaders"
	segRising     mvpCommerceSegment = "rising"
	segDeclining  mvpCommerceSegment = "declining"
	segLowStock   mvpCommerceSegment = "low_stock"
	segOverstock  mvpCommerceSegment = "overstock"
	segAdWaste    mvpCommerceSegment = "ad_waste"
	segPriceRisk  mvpCommerceSegment = "price_risk"
	segStableTail mvpCommerceSegment = "stable_tail"
)

type mvpCatalogEntry struct {
	NameRU         string
	Category       mvpCommerceCategory
	CategoryNameRU string
	Brand          string
	PackageSize    string
	ColorMaterial  string
	OfferPrefix    string // e.g. HOME-KITCHEN
	BasePrice      float64
}

// mvpProductCatalog is a rotating list of realistic Ozon-style home goods (RU).
var mvpProductCatalog = []mvpCatalogEntry{
	{"Органайзер для кухни 3 секции", catKitchenStorage, "Хранение на кухне", "HomeLine", "42×18×12 см", "пластик серый", "HOME-KITCHEN", 890},
	{"Контейнер пищевой герметичный 1.2 л", catKitchenStorage, "Хранение на кухне", "FreshBox", "1.2 л", "пластик прозрачный", "HOME-KITCHEN", 420},
	{"Набор полотенец хлопок 50×90", catHomeTextile, "Текстиль для дома", "SoftCotton", "50×90 см, 4 шт", "хлопок белый", "HOME-TEXT", 1150},
	{"Чехол для одежды 60×100", catHomeOrganization, "Организация пространства", "Guard", "60×100 см", "нетканое полотно", "HOME-ORG", 360},
	{"Щётка для уборки с дозатором", catCleaning, "Уборка", "CleanPro", "1 шт", "пластик/нейлон", "HOME-CLEAN", 520},
	{"LED-лампа настольная складная", catLightDecor, "Освещение", "LumiDesk", "5 Вт", "пластик белый", "HOME-LIGHT", 1290},
	{"Коврик влаговпитывающий для прихожей", catHomeTextile, "Текстиль для дома", "DryMat", "50×80 см", "микрофибра серый", "HOME-TEXT", 780},
	{"Набор крючков самоклеящихся", catBathroom, "Ванная комната", "StickHook", "6 шт", "пластик прозрачный", "HOME-BATH", 290},
	{"Салфетки микрофибра 5 шт", catCleaning, "Уборка", "MicroWipe", "30×30 см", "микрофибра", "HOME-CLEAN", 340},
	{"Полка-органайзер для ванной", catBathroom, "Ванная комната", "BathRack", "35×12 см", "пластик белый", "HOME-BATH", 640},
	{"Набор банок для специй", catKitchenStorage, "Хранение на кухне", "SpiceKit", "6×0.2 л", "стекло/бамбук", "HOME-KITCHEN", 990},
	{"Вешалки пластиковые 10 шт", catHomeOrganization, "Организация пространства", "HangSet", "10 шт", "пластик чёрный", "HOME-ORG", 310},
	{"Корзина для белья складная", catHomeOrganization, "Организация пространства", "LaundryFold", "45 л", "полиэстер бежевый", "HOME-ORG", 1490},
	{"Органайзер для косметики", catBathroom, "Ванная комната", "CosmoBox", "24×16×12 см", "акрил прозрачный", "HOME-BATH", 720},
	{"Диспенсер для моющего средства", catKitchenStorage, "Хранение на кухне", "SoapPress", "350 мл", "пластик матовый", "HOME-KITCHEN", 410},
	{"Разделитель для ящика кухонного", catKitchenStorage, "Хранение на кухне", "DrawerFit", "набор 4 шт", "пластик белый", "HOME-KITCHEN", 560},
	{"Подставка для столовых приборов", catKitchenStorage, "Хранение на кухне", "UtensilTree", "1 шт", "бамбук", "HOME-KITCHEN", 680},
	{"Сушилка для посуды над раковиной", catKitchenStorage, "Хранение на кухне", "DryBridge", "85 см", "нержавеющая сталь", "HOME-KITCHEN", 1590},
	{"Коврик противоскользящий для ванной", catBathroom, "Ванная комната", "SafeStep", "40×60 см", "ПВХ синий", "HOME-BATH", 450},
	{"Штора для ванной влагостойкая", catBathroom, "Ванная комната", "AquaCurtain", "180×200 см", "полиэстер", "HOME-BATH", 1890},
	{"Ночник сенсорный с диммером", catLightDecor, "Освещение", "NightEase", "1 шт", "пластик белый", "HOME-LIGHT", 590},
	{"Гирлянда LED на батарейках", catLightDecor, "Освещение", "FairyLED", "5 м", "медный провод тёплый", "HOME-LIGHT", 720},
	{"Плед флисовый 150×200", catHomeTextile, "Текстиль для дома", "CozyFleece", "150×200 см", "флис графит", "HOME-TEXT", 1690},
	{"Наволочка сатин 50×70", catHomeTextile, "Текстиль для дома", "SilkTouch", "2 шт", "сатин молочный", "HOME-TEXT", 890},
	{"Швабра с отжимом и ведром", catCleaning, "Уборка", "MopPro", "комплект", "пластик/микрофибра", "HOME-CLEAN", 2190},
	{"Перчатки хозяйственные латексные", catCleaning, "Уборка", "HandSafe", "3 пары", "латекс", "HOME-CLEAN", 240},
	{"Пылесос ручной для мебели", catCleaning, "Уборка", "MiniVac", "USB", "пластик чёрный", "HOME-CLEAN", 2490},
	{"Этажерка на колёсиках 4 полки", catHomeOrganization, "Организация пространства", "RollShelf", "42×32×85 см", "металл/пластик", "HOME-ORG", 3290},
	{"Короб для хранения с крышкой", catHomeOrganization, "Организация пространства", "BoxStore", "33 л", "полипропилен", "HOME-ORG", 690},
	{"Подставка под горячее бамбук", catKitchenStorage, "Хранение на кухне", "HeatPad", "набор 4 шт", "бамбук", "HOME-KITCHEN", 480},
	{"Термос 750 мл нержавейка", catKitchenStorage, "Хранение на кухне", "ThermoWalk", "750 мл", "сталь матовая", "HOME-KITCHEN", 1390},
	{"Фильтр-кувшин с картриджем", catKitchenStorage, "Хранение на кухне", "AquaJug", "2.5 л", "пластик прозрачный", "HOME-KITCHEN", 1190},
	{"Скалка с антипригарным покрытием", catKitchenStorage, "Хранение на кухне", "BakeRoll", "40 см", "силикон/дерево", "HOME-KITCHEN", 560},
	{"Коврик под кресло защитный", catHomeOrganization, "Организация пространства", "FloorGuard", "90×120 см", "ПВХ прозрачный", "HOME-ORG", 990},
	{"Подушка ортопедическая 40×60", catHomeTextile, "Текстиль для дома", "OrthoRest", "40×60 см", "пена memory", "HOME-TEXT", 2490},
	{"Занавеска blackout 140×260", catHomeTextile, "Текстиль для дома", "NightBlock", "140×260 см", "полиэстер тёмно-синий", "HOME-TEXT", 2190},
	{"Светильник настенный бра 1×E14", catLightDecor, "Освещение", "WallGlow", "1×E14", "металл чёрный", "HOME-LIGHT", 1790},
	{"Таймер механический для кухни", catKitchenStorage, "Хранение на кухне", "CookTick", "60 мин", "пластик белый", "HOME-KITCHEN", 320},
	{"Дозатор для масла и уксуса", catKitchenStorage, "Хранение на кухне", "OilTwin", "2×200 мл", "стекло", "HOME-KITCHEN", 540},
	{"Сушилка для белья напольная", catHomeOrganization, "Организация пространства", "AirStand", "18 м сушки", "сталь/пластик", "HOME-ORG", 2790},
	{"Ведро с отжимом 12 л", catCleaning, "Уборка", "SpinBucket", "12 л", "пластик синий", "HOME-CLEAN", 890},
	{"Щётка для одежды липкая", catCleaning, "Уборка", "LintRoll", "80 листов", "бумага/пластик", "HOME-CLEAN", 260},
	{"Крючки настенные металлические", catBathroom, "Ванная комната", "MetalHook", "4 шт", "матовый никель", "HOME-BATH", 430},
	{"Полка угловая для душа", catBathroom, "Ванная комната", "CornerShelf", "1 шт", "нержавейка", "HOME-BATH", 1190},
	{"Лоток для столовых приборов", catKitchenStorage, "Хранение на кухне", "TraySoft", "38×28 см", "пластик серый", "HOME-KITCHEN", 470},
	{"Контейнер для крупы 2.0 л", catKitchenStorage, "Хранение на кухне", "GrainLock", "2.0 л", "пластик", "HOME-KITCHEN", 510},
	{"Корзина плетёная для хранения", catHomeOrganization, "Организация пространства", "WovenBin", "30×25×20 см", "ротанг", "HOME-ORG", 1390},
	{"Плед детский 100×140", catHomeTextile, "Текстиль для дома", "BabySoft", "100×140 см", "флис", "HOME-TEXT", 990},
	{"Светильник подсветки кухонного шкафа", catLightDecor, "Освещение", "UnderGlow", "50 см", "алюминий", "HOME-LIGHT", 1590},
	{"Удлинитель сетевой с заземлением", catLightDecor, "Освещение", "PowerSafe", "3 м, 6 розеток", "пластик белый", "HOME-LIGHT", 890},
	{"Сушилка для обуви электрическая", catHomeOrganization, "Организация пространства", "ShoeDry", "пара", "пластик", "HOME-ORG", 1990},
	{"Мыльница настольная керамическая", catBathroom, "Ванная комната", "CeramicSoap", "1 шт", "керамика белая", "HOME-BATH", 380},
	{"Держатель для туалетной бумаги", catBathroom, "Ванная комната", "PaperHold", "1 шт", "нержавейка", "HOME-BATH", 520},
	{"Скребок для стеклокерамики", catCleaning, "Уборка", "GlassSafe", "1 шт", "пластик/резина", "HOME-CLEAN", 310},
	{"Пылесборник для робота-пылесоса", catCleaning, "Уборка", "RoboBag", "3 шт", "неткань", "HOME-CLEAN", 690},
	{"Коврик придверный резиновый", catHomeTextile, "Текстиль для дома", "DoorMat", "45×75 см", "резина чёрная", "HOME-TEXT", 590},
	{"Органайзер подвесной в шкаф", catHomeOrganization, "Организация пространства", "ClosetHang", "6 полок", "ткань/картон", "HOME-ORG", 790},
	{"Разделитель для кастрюль", catKitchenStorage, "Хранение на кухне", "PotGuard", "3 шт", "силикон", "HOME-KITCHEN", 430},
	{"Терка многофункциональная", catKitchenStorage, "Хранение на кухне", "GrateAll", "набор", "нержавейка", "HOME-KITCHEN", 650},
	{"Форма для выпечки силиконовая", catKitchenStorage, "Хранение на кухне", "BakeForm", "26 см", "силикон", "HOME-KITCHEN", 720},
	{"Сушилка для белья настенная", catHomeOrganization, "Организация пространства", "WallDry", "1 м", "алюминий", "HOME-ORG", 1890},
	{"Светильник напольный торшер", catLightDecor, "Освещение", "FloorTorch", "E27", "металл/ткань", "HOME-LIGHT", 4290},
	{"Постельное бельё 1.5 спальное", catHomeTextile, "Текстиль для дома", "SleepSet", "1.5 сп", "поплин", "HOME-TEXT", 3290},
	{"Ковёр короткий ворс 120×170", catHomeTextile, "Текстиль для дома", "SoftRug", "120×170 см", "полипропилен", "HOME-TEXT", 2490},
	{"Ведро складное силиконовое", catCleaning, "Уборка", "FoldBucket", "10 л", "силикон", "HOME-CLEAN", 790},
	{"Швабра паровая насадки 2 шт", catCleaning, "Уборка", "SteamMop", "набор", "микрофибра", "HOME-CLEAN", 420},
	{"Держатель для зубных щёток", catBathroom, "Ванная комната", "BrushHold", "семейный", "пластик", "HOME-BATH", 360},
	{"Коврик для ванны антибактериальный", catBathroom, "Ванная комната", "AntiBath", "50×80 см", "силикон", "HOME-BATH", 890},
	{"Лампа USB для чтения прищепка", catLightDecor, "Освещение", "ClipRead", "1 Вт", "пластик", "HOME-LIGHT", 490},
	{"Набор вешалок деревянных", catHomeOrganization, "Организация пространства", "WoodHang", "8 шт", "дерево", "HOME-ORG", 890},
	{"Контейнер для мусора с педалью", catHomeOrganization, "Организация пространства", "PedalBin", "20 л", "сталь матовая", "HOME-ORG", 2490},
	{"Подставка для ножей магнитная", catKitchenStorage, "Хранение на кухне", "KnifeMag", "40 см", "бамбук/магнит", "HOME-KITCHEN", 1190},
	{"Бутылка для воды спортивная", catKitchenStorage, "Хранение на кухне", "SportBottle", "750 мл", "тритан", "HOME-KITCHEN", 640},
	{"Органайзер для обуви 4 яруса", catHomeOrganization, "Организация пространства", "ShoeTower", "4 яруса", "пластик", "HOME-ORG", 1590},
	{"Плед с рукавами", catHomeTextile, "Текстиль для дома", "SleeveBlanket", "универсальный", "флис", "HOME-TEXT", 1390},
	{"Светильник ночник детский", catLightDecor, "Освещение", "KidMoon", "1 шт", "пластик", "HOME-LIGHT", 690},
	{"Сушилка для посуды настольная", catKitchenStorage, "Хранение на кухне", "DeskDry", "42×32 см", "пластик", "HOME-KITCHEN", 890},
	{"Коврик под мышь большой", catLightDecor, "Освещение", "DeskMat", "90×40 см", "ткань", "HOME-LIGHT", 590},
	{"Контейнер для яиц 24 шт", catKitchenStorage, "Хранение на кухне", "EggSafe", "24 яйца", "пластик", "HOME-KITCHEN", 390},
	{"Набор форм для льда с крышкой", catKitchenStorage, "Хранение на кухне", "IceTray", "2 формы", "силикон", "HOME-KITCHEN", 340},
	{"Корзина для игрушек на завязках", catHomeOrganization, "Организация пространства", "ToyBag", "45 л", "хлопок", "HOME-ORG", 790},
	{"Подушка декоративная 45×45", catHomeTextile, "Текстиль для дома", "DecorPuff", "45×45 см", "велюр", "HOME-TEXT", 690},
	{"Светильник спот 3×GU10", catLightDecor, "Освещение", "SpotTrio", "3×GU10", "металл хром", "HOME-LIGHT", 2190},
	{"Швабра для окон телескопическая", catCleaning, "Уборка", "WinReach", "1.3 м", "пластик", "HOME-CLEAN", 1190},
	{"Ведро с разделителем двойное", catCleaning, "Уборка", "TwinBucket", "2×7 л", "пластик", "HOME-CLEAN", 990},
	{"Набор емкостей для ванной", catBathroom, "Ванная комната", "BathSet", "3 шт", "керамика", "HOME-BATH", 1290},
	{"Зеркало косметическое настольное", catBathroom, "Ванная комната", "TableMirror", "увеличение", "стекло/пластик", "HOME-BATH", 590},
	{"Лоток для сушки посуды раздвижной", catKitchenStorage, "Хранение на кухне", "SlideDry", "до 85 см", "пластик", "HOME-KITCHEN", 1490},
	{"Контейнер вакуумный для кофе", catKitchenStorage, "Хранение на кухне", "CoffeeVac", "500 г", "стекло", "HOME-KITCHEN", 890},
	{"Короб архивный с крышкой", catHomeOrganization, "Организация пространства", "ArchiveBox", "40 л", "картон", "HOME-ORG", 420},
	{"Одеяло всесезонное 200×220", catHomeTextile, "Текстиль для дома", "AllYear", "200×220 см", "микрофибра", "HOME-TEXT", 2890},
	{"Бра с выключателем на планке", catLightDecor, "Освещение", "WallTwin", "2×E14", "металл", "HOME-LIGHT", 1390},
	{"Пылесос вертикальный беспроводной", catCleaning, "Уборка", "StickVac", "22 кПа", "пластик", "HOME-CLEAN", 8990},
	{"Средство для мытья полов 2 л", catCleaning, "Уборка", "FloorClean", "2 л", "пластик", "HOME-CLEAN", 390},
	{"Коврик для йоги 6 мм", catHomeTextile, "Текстиль для дома", "YogaMat", "183×61 см", "TPE", "HOME-TEXT", 1290},
	{"Полка навесная без сверления", catBathroom, "Ванная комната", "NoDrill", "30 см", "пластик", "HOME-BATH", 740},
	{"Светильник с датчиком движения", catLightDecor, "Освещение", "MotionLED", "USB-C", "пластик белый", "HOME-LIGHT", 990},
	{"Органайзер для проводов", catHomeOrganization, "Организация пространства", "CableBox", "24×11 см", "пластик", "HOME-ORG", 540},
	{"Подставка для ноутбука складная", catLightDecor, "Освещение", "LapStand", "15.6\"", "алюминий", "HOME-LIGHT", 2190},
	{"Термометр кухонный цифровой", catKitchenStorage, "Хранение на кухне", "TempChef", "щуп", "пластик", "HOME-KITCHEN", 480},
	{"Мельница для специй 2 в 1", catKitchenStorage, "Хранение на кухне", "SpiceMill", "керамика", "акрил", "HOME-KITCHEN", 760},
	{"Корзина для мелочей на стол", catHomeOrganization, "Организация пространства", "DeskBin", "15×10 см", "металл сетка", "HOME-ORG", 360},
	{"Простыня на резинке 160×200", catHomeTextile, "Текстиль для дома", "FitSheet", "160×200", "хлопок", "HOME-TEXT", 1590},
	{"Светильник настольный с аккумулятором", catLightDecor, "Освещение", "DeskCharge", "4000 mAh", "пластик", "HOME-LIGHT", 1790},
	{"Швабра с распылителем", catCleaning, "Уборка", "SprayMop", "комплект", "пластик", "HOME-CLEAN", 1490},
	{"Сушилка для белья потолочная", catHomeOrganization, "Организация пространства", "CeilDry", "1.8 м", "сталь", "HOME-ORG", 3190},
	{"Дозатор для мыла сенсорный", catBathroom, "Ванная комната", "FoamSense", "280 мл", "пластик", "HOME-BATH", 1990},
	{"Коврик для ванной мягкий", catBathroom, "Ванная комната", "SoftBath", "50×80 см", "микрофибра", "HOME-BATH", 640},
	{"Лампа настольная с беспроводной зарядкой", catLightDecor, "Освещение", "ChargeLamp", "10 Вт", "пластик", "HOME-LIGHT", 2490},
	{"Органайзер для холодильника набор", catKitchenStorage, "Хранение на кухне", "FridgeKit", "8 шт", "пластик", "HOME-KITCHEN", 990},
	{"Контейнер для снеков набор", catKitchenStorage, "Хранение на кухне", "SnackPack", "4×0.4 л", "пластик", "HOME-KITCHEN", 620},
	{"Полка для специй настенная", catKitchenStorage, "Хранение на кухне", "SpiceRack", "2 яруса", "металл", "HOME-KITCHEN", 1190},
	{"Корзина для документов А4", catHomeOrganization, "Организация пространства", "DocBin", "A4", "картон", "HOME-ORG", 290},
	{"Набор полотенец кухонных 3 шт", catHomeTextile, "Текстиль для дома", "KitchenTowel", "40×60 см", "хлопок", "HOME-TEXT", 540},
	{"Светильник подвесной одиночный", catLightDecor, "Освещение", "PendOne", "E27", "металл", "HOME-LIGHT", 1590},
	{"Щётка для ковров жёсткая", catCleaning, "Уборка", "CarpetBrush", "1 шт", "дерево/щетина", "HOME-CLEAN", 480},
	{"Средство для мытья посуды 1 л", catCleaning, "Уборка", "DishSoap", "1 л", "пластик", "HOME-CLEAN", 320},
	{"Набор крючков на липучке прозрачных", catBathroom, "Ванная комната", "ClearHook", "8 шт", "пластик", "HOME-BATH", 260},
	{"Стакан для зубных щёток", catBathroom, "Ванная комната", "GlassCup", "1 шт", "стекло", "HOME-BATH", 290},
	{"Светодиодная лента 5 м RGB", catLightDecor, "Освещение", "RGBStrip", "5 м", "силикон", "HOME-LIGHT", 1890},
	{"Органайзер для ремней и галстуков", catHomeOrganization, "Организация пространства", "BeltBox", "6 ячеек", "ткань", "HOME-ORG", 590},
	{"Подушка для беременных U-образная", catHomeTextile, "Текстиль для дома", "MomU", "универсальная", "холлофайбер", "HOME-TEXT", 3990},
	{"Светильник точечный встраиваемый", catLightDecor, "Освещение", "DownSpot", "GX53", "металл", "HOME-LIGHT", 390},
	{"Веник для улицы с черенком", catCleaning, "Уборка", "YardBroom", "1.4 м", "пластик/щетина", "HOME-CLEAN", 690},
	{"Сушилка для белья напольная X-образная", catHomeOrganization, "Организация пространства", "XDry", "18 м", "сталь", "HOME-ORG", 2290},
	{"Полка для душа угловая стекло", catBathroom, "Ванная комната", "GlassCorner", "8 мм", "стекло", "HOME-BATH", 2490},
	{"Лампа настольная с регулировкой цвета", catLightDecor, "Освещение", "TuneDesk", "9 Вт", "пластик", "HOME-LIGHT", 1990},
	{"Контейнер для хлеба", catKitchenStorage, "Хранение на кухне", "BreadBox", "5 л", "пластик", "HOME-KITCHEN", 790},
	{"Разделители для полок набор", catHomeOrganization, "Организация пространства", "ShelfDiv", "6 шт", "пластик", "HOME-ORG", 430},
	{"Плед клетчатый 130×170", catHomeTextile, "Текстиль для дома", "CheckPlaid", "130×170 см", "акрил", "HOME-TEXT", 1190},
	{"Светильник ночник проектор звёзд", catLightDecor, "Освещение", "StarProj", "USB", "пластик", "HOME-LIGHT", 890},
	{"Швабра с отжимом плоская", catCleaning, "Уборка", "FlatMop", "комплект", "микрофибра", "HOME-CLEAN", 1790},
	{"Коврик придверный влаговпитывающий", catHomeTextile, "Текстиль для дома", "AbsorbMat", "60×90 см", "полиэстер", "HOME-TEXT", 990},
	{"Держатель для фена настенный", catBathroom, "Ванная комната", "DryerHold", "1 шт", "металл", "HOME-BATH", 640},
	{"Светильник бра с USB", catLightDecor, "Освещение", "USBWall", "1×E27", "металл", "HOME-LIGHT", 1690},
	{"Органайзер для кабелей самоклеящийся", catHomeOrganization, "Организация пространства", "CableClip", "10 шт", "силикон", "HOME-ORG", 340},
	{"Кастрюля с крышкой 3 л", catKitchenStorage, "Хранение на кухне", "PotThree", "3 л", "нержавейка", "HOME-KITCHEN", 2490},
	{"Скалка кондитерская регулируемая", catKitchenStorage, "Хранение на кухне", "RollAdjust", "1 шт", "пластик", "HOME-KITCHEN", 890},
	{"Корзина для бумаг офисная", catHomeOrganization, "Организация пространства", "PaperBin", "12 л", "пластик", "HOME-ORG", 490},
	{"Покрывало стеганое 220×240", catHomeTextile, "Текстиль для дома", "QuiltCover", "220×240 см", "микрофибра", "HOME-TEXT", 2190},
	{"Светильник потолочный LED 24 Вт", catLightDecor, "Освещение", "CeilLED", "24 Вт", "акрил", "HOME-LIGHT", 1890},
	{"Ведро с воронкой для мытья окон", catCleaning, "Уборка", "WindowKit", "комплект", "пластик", "HOME-CLEAN", 590},
	{"Сушилка для белья на радиатор", catHomeOrganization, "Организация пространства", "RadDry", "55 см", "металл", "HOME-ORG", 690},
	{"Коврик для ванной на присосках", catBathroom, "Ванная комната", "SuctionMat", "53×53 см", "ПВХ", "HOME-BATH", 520},
	{"Лампа настольная с часами", catLightDecor, "Освещение", "ClockLamp", "LED", "пластик", "HOME-LIGHT", 1390},
	{"Органайзер для специй выдвижной", catKitchenStorage, "Хранение на кухне", "SpicePull", "3 яруса", "дерево", "HOME-KITCHEN", 2190},
	{"Контейнер для хранения обуви", catHomeOrganization, "Организация пространства", "ShoeBox", "33×23×14 см", "пластик", "HOME-ORG", 360},
	{"Наволочки декоративные 45×45 2 шт", catHomeTextile, "Текстиль для дома", "DecorPair", "45×45 см", "лен", "HOME-TEXT", 790},
	{"Светильник уличный настенный", catLightDecor, "Освещение", "OutdoorWall", "E27", "алюминий", "HOME-LIGHT", 2490},
	{"Швабра для ламината с насадкой", catCleaning, "Уборка", "LamMop", "1 шт", "микрофибра", "HOME-CLEAN", 690},
	{"Ведро круглое 10 л", catCleaning, "Уборка", "Round10", "10 л", "пластик", "HOME-CLEAN", 340},
	{"Полка для ванной на присосках", catBathroom, "Ванная комната", "SuctionShelf", "1 ярус", "пластик", "HOME-BATH", 890},
	{"Светильник настольный с лупой", catLightDecor, "Освещение", "MagniLamp", "3 диоптрии", "металл", "HOME-LIGHT", 3290},
	{"Органайзер для мелочей 16 ячеек", catHomeOrganization, "Организация пространства", "BitBox", "16 ячеек", "пластик", "HOME-ORG", 520},
	{"Пододеяльник 1.5 сп на молнии", catHomeTextile, "Текстиль для дома", "DuvetZip", "1.5 сп", "сатин", "HOME-TEXT", 1790},
	{"Светильник ночник с таймером", catLightDecor, "Освещение", "TimerNight", "1 шт", "пластик", "HOME-LIGHT", 590},
	{"Средство для чистки плит 500 мл", catCleaning, "Уборка", "StoveClean", "500 мл", "пластик", "HOME-CLEAN", 410},
	{"Сушилка для белья настенная складная", catHomeOrganization, "Организация пространства", "WallFold", "1 м", "алюминий", "HOME-ORG", 2490},
	{"Крючок для полотенца кольцо", catBathroom, "Ванная комната", "TowelRing", "1 шт", "нержавейка", "HOME-BATH", 480},
	{"Лампа настольная детская", catLightDecor, "Освещение", "KidLamp", "7 Вт", "пластик", "HOME-LIGHT", 990},
	{"Органайзер для столовых приборов в ящик", catKitchenStorage, "Хранение на кухне", "DrawerUtensil", "широкий", "бамбук", "HOME-KITCHEN", 1190},
	{"Контейнер для лука чеснока", catKitchenStorage, "Хранение на кухне", "AlliumBox", "2 отделения", "пластик", "HOME-KITCHEN", 340},
	{"Корзина для журналов напольная", catHomeOrganization, "Организация пространства", "MagRack", "1 шт", "металл", "HOME-ORG", 990},
	{"Покрывало диванное 180×220", catHomeTextile, "Текстиль для дома", "SofaThrow", "180×220 см", "акрил", "HOME-TEXT", 1690},
	{"Светильник бра с выключателем шнуром", catLightDecor, "Освещение", "CordWall", "E14", "ткань/металл", "HOME-LIGHT", 1490},
	{"Швабра для плитки с насадкой", catCleaning, "Уборка", "TileMop", "1 шт", "микрофибра", "HOME-CLEAN", 540},
	{"Ведро овальное 13 л", catCleaning, "Уборка", "Oval13", "13 л", "пластик", "HOME-CLEAN", 420},
	{"Набор для ванной керамика 4 предмета", catBathroom, "Ванная комната", "CeramicSet", "4 шт", "керамика", "HOME-BATH", 1590},
	{"Светильник настольный складной LED", catLightDecor, "Освещение", "FoldLED", "5 Вт", "пластик", "HOME-LIGHT", 1290},
}

func categoryDescriptionID(cat mvpCommerceCategory, sellerAccountID int64) int64 {
	h := int64(0)
	for _, c := range string(cat) {
		h = h*31 + int64(c)
	}
	if h < 0 {
		h = -h
	}
	return 17_000_000 + (h % 50_000) + sellerAccountID%100
}

func commerceSegmentForIndex(i, n int) mvpCommerceSegment {
	order := []mvpCommerceSegment{
		segLeaders, segRising, segDeclining, segLowStock,
		segOverstock, segAdWaste, segPriceRisk, segStableTail,
	}
	if n < len(order) {
		return order[i%len(order)]
	}
	if i < len(order) {
		return order[i]
	}
	// Weighted tail: more leaders + stable for revenue / tail mass
	switch i % 11 {
	case 0, 1, 2:
		return segLeaders
	case 3, 4:
		return segRising
	case 5:
		return segDeclining
	case 6:
		return segLowStock
	case 7:
		return segOverstock
	case 8:
		return segAdWaste
	case 9:
		return segPriceRisk
	default:
		return segStableTail
	}
}
