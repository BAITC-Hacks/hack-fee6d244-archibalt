# Рейтинг готовности бизнес-задачи: аналоги, критерии, AI-извлечение, демо

Дата: 23.09.2026 (Астана). Автор: Fable (веб-проверено, ссылки открыты 23.09.2026, кроме отмеченных). Контекст: `TASK.md` (AI Sana Challenge Hub), формула в `internal/rating/rating.go`. Пометки: **Факт** — из источника; **Вывод** — наша интерпретация; **[не открылся]** — источник вернул 403/ошибку, данные из поисковой выдачи.

## 1. Аналоги «рейтинг полноты публикующей стороны»

**Факты**
- LinkedIn Profile level meter: измеряет только полноту, 3 уровня (Beginner → Intermediate после 4 разделов → All-star после всех 7: фото, локация, отрасль, образование, должность, навыки, summary); под шкалой «prompts to move to the next level»; уровень виден только владельцу; LinkedIn заявляет рост discoverability в поиске. [LinkedIn Help, обновлено ~7 мес. назад](https://www.linkedin.com/help/linkedin/answer/a594698). Сторонний разбор 2026: метр «scores completeness, not quality», в ранжировании Recruiter не участвует, ранжирует релевантность ключевых слов. [slategit.com](https://slategit.com/blog/linkedin-all-star-profile-strength-guide)
- Airbnb: 9 официальных факторов ранжирования (цена, quality, popularity, availability, Instant Book, response time, badges, accuracy, restrictions); «listings with more details get up to 20% more bookings», минимум 20 фото для полноты. Полнота — один из факторов, которые хост может поднять без отзывов. [PriceLabs 2026](https://hello.pricelabs.co/blog/airbnb-ranking-algorithm/), [BNBCalc 2026](https://www.bnbcalc.com/blog/research/airbnb-listing-optimization-11-ways)
- Upwork: обязательные поля job post — title, description, skills, experience level, budget/type; детальное описание «helps the algorithm show your job to the right freelancers». [Upwork Help](https://support.upwork.com/hc/en-us/articles/211063408-How-to-post-a-job) [не открылся, 403], [gigradar](https://gigradar.io/blog/how-to-post-a-job-on-upwork)
- Riipen (Level UP, обновлено 10.10.2025; FuturePath, обновлено 03.03.2026): проект обязан содержать goal, scope, expectations, deliverables, required skills, resources, mentorship plan; «must describe a fully-developed project, not a job description»; есть чеклист готовности из 7 пунктов перед публикацией; лимит часов (60 / 10–420) и до 2–4 студентов. [Level UP](https://help.riipen.com/en/articles/12557823-creating-a-successful-level-up-project-for-employers), [FuturePath](https://help.riipen.com/en/articles/11101502-creating-a-successful-riipen-futurepath-project-for-employers)
- Kaggle host docs: обязательны problem statement с мотивацией, data description, evaluation metric и rubric до запуска; «contest of skill»: участники должны понимать критерии оценки заранее; узкий scope упрощает судейство. [Kaggle Competitions Setup](https://www.kaggle.com/docs/competitions-setup)
- Паттерн Completeness Meter (ui-patterns): показывать следующий шаг и выгоду, праздновать достижение; низкий стартовый процент (~30%) демотивирует, около 80% лучше стимулирует. [ui-patterns.com](https://ui-patterns.com/patterns/CompletenessMeter)
- Goodhart: «когда мера становится целью, она перестаёт быть мерой»; полнотные бары стимулируют заполнение ради процента, а не ради содержания. [Splunk](https://www.splunk.com/en_us/blog/learn/goodharts-law.html), [WFM Labs](https://wiki.wfmlabs.org/wiki/Goodhart's_Law_and_Metric_Gaming)

**Выводы**
- Все зрелые платформы измеряют полноту чеклистом фиксированных разделов и показывают «что добавить для следующего уровня». Ровно это требует ТЗ §3–4. Ни одна не претендует на оценку качества текста; честно называть наш балл «готовность», не «качество».
- Известная слабость: заполнить можно чем угодно. Riipen и Kaggle решают это признаками содержания (deliverables, metric, hours), а не длиной.
- Влияние на ранжирование у аналогов есть, но мягкое (один фактор из многих). У нас по ТЗ рейтинг = позиция, это сильнее, значит признаки полноты должны быть строже длины.

**Применить у нас**
- Оставить признаки на поле (число/срок в критериях, артефакт в результате, контакт в связи). Это уже в `rating.go`. Добавить в подсказку missing формулировку «что именно» + «+N баллов» + пример ответа (как Riipen/LinkedIn prompts).
- Подпись под шкалой: «Готовность к работе со студентами, не оценка компании» (ТЗ §4, и это снимает вопрос жюри про Goodhart).
- Показать «следующий уровень: до "готовая" не хватает 12 баллов» — паттерн completeness meter в чистом виде.

## 2. Что делает бизнес-бриф хорошим для студенческого проекта

**Факты**
- UW Engineering Capstone proposal 2025–2026: поля формы — project description, motivation and relevance to sponsor; design parameters and performance criteria; desired outcomes and deliverables; ожидаемые затраты и ресурсы спонсора; NDA yes/no; обязательные роли — project coordinator, technical mentor, billing contact с телефоном и email. [capstone-proposal.engr.uw.edu](https://capstone-proposal.engr.uw.edu/)
- UChicago MS-ADS Capstone Sponsor Guide 2025: proposal = title, problem description, expected workload, data sources, tools, deliverables, contact; обязательства спонсора — «data due to student team within two weeks», регулярные встречи, point of contact, data-sharing agreement; успех фиксируется charter и sponsor sign-off. [PDF](https://datascience.uchicago.edu/wp-content/uploads/2024/10/MS-ADS-Capstone-Sponsor-Guide-2025.pdf)
- Riipen: отказ в публикации, если это «job description», а не проект с deliverables (см. §1).

**Выводы**
- Университетские программы требуют те же 10 полей, что ТЗ §3, плюс три вещи, которых у бизнеса чаще всего нет: срок предоставления данных, регулярность встреч, именованный контакт-ментор. Именно «данные» и «связь с бизнесом» — типичные пустоты.
- «Ожидаемый результат» везде формулируется как артефакт (код, отчёт, презентация), а «критерии успеха» как измеримые параметры производительности.

**Применить у нас**
- Уточняющие вопросы mock/AI приоритизировать: data (20) → success_criteria (15) → contact+interaction_format (10) — это поля, которые бизнес пропускает чаще всего и которые дороже всего в рейтинге.
- Признак «полно» для data: упоминание срока/способа передачи («выгрузка», «доступ», «в течение недели») даёт полный балл; просто «есть данные» — половина. Это соответствует «data due within two weeks».
- Признак для interaction_format: периодичность («раз в неделю», «созвон по пятницам») — полный балл.

## 3. Казахстанский контекст (AI Sana)

**Факты**
- AI-Sana: программа МНВО, базовый этап прошли ~540 тыс. из 650 тыс. студентов (Astana Hub, Google, Coursera, Huawei); второй этап — отбор 100 тыс.; платформа Alem.AI открыта 1 октября 2025 для акселерации студенческих проектов; цель 1,5 тыс. проектов к концу 2026. [profit.kz, 15.10.2025](https://profit.kz/news/71969/540-tisyach-kazahstanskih-studentov-osvoili-bazovie-naviki-II-po-programme-AI-Sana/)
- Второй этап: 87 тыс. студентов, 1 500 менторов, 105 вузов, «цифровые решения для реальных отраслевых задач», акселерация 1 500 проектов через Alem.AI. [kazpravda.kz, 2026](https://kazpravda.kz/n/kazahstanskie-studenty-razrabotali-bolee-400-ii-proektov-v-ramkah-programmy-ai-sana/) [не открылся, 403; из выдачи]
- Публичного описания «AI Sana Challenge Hub» как действующей платформы каталога бизнес-задач в поиске не найдено (запросы на русском, 23.09.2026). **[не подтверждено]** Вероятно, наш кейс и есть прототип такого хаба.

**Выводы**
- Заказчик (МНВО/AI Sana) уже имеет массу студентов и менторов, но нет описанного публично механизма, как бизнес формулирует задачи. Наш продукт закрывает вход воронки: качество задачи до того, как 87 тыс. студентов её увидят.
- Язык заказчика: «реальные отраслевые задачи», «акселерация», «менторы», «Alem.AI». В демо и README использовать эти термины, а не «маркетплейс».

**Применить у нас**
- В первом экране: «Задача от бизнеса → готова для команд AI-Sana». Отрасли в seed брать из приоритетов программы: агро, водные ресурсы, энергетика, транспорт, госуслуги.
- В README раздел «Как встраивается в AI-Sana»: каталог = вход для студенческих команд второго этапа, отклик = заявка в акселерацию. Одна фраза, без обещаний.

## 4. Structured extraction без галлюцинаций

**Факты**
- OpenAI Structured Outputs: `strict: true` гарантирует соответствие схеме (нет пропущенных required, нет невалидных enum); отсутствующее значение выражается через `type: ["string","null"]` или `anyOf`, поле остаётся required; **не гарантирует фактическую верность** — модель может выдать правдоподобную ложь внутри валидной схемы. [OpenAI docs](https://developers.openai.com/api/docs/guides/structured-outputs)
- Практика 2026: JSON mode считается legacy, для извлечения — strict json_schema; ограничения enum/min/max/pattern сужают пространство выдумок; schema-first. [ergini 2026](https://ergini.com/blog/openai-structured-outputs), [codewords](https://www.codewords.ai/blog/openai-structured-outputs-json-schema)

**Выводы**
- Схема закрывает форму, не содержание. ТЗ §5 «ИИ не должен добавлять факты» требует отдельного слоя проверки поверх схемы; наш `Guard` (стем-пересечение + числа только из источника) — правильная и объяснимая жюри мера.
- Nullable-поля в схеме лучше пустых строк: модель явно говорит «данных нет», а не изобретает.

**Применить у нас**
- В схеме Card сделать поля `["string","null"]`, в промпте — «null, если пользователь не сообщил». В коде null → "". Сейчас у нас string; поправить в `internal/ai/prompts.go` (владелец Абылай).
- На странице `/ai` показать три вещи, которые проверяет жюри по §5: промпт, схему, пример «выдуманный бюджет вырезан Guard». Это готовый ответ на «AI функция 10 баллов».
- В UI помечать поля, заполненные AI, иконкой и требовать явного подтверждения (уже в контракте: confirm).

## 5. Что жюри хвалит и ругает в marketplace-MVP

**Факты**
- Судья-практик (dev.to, 3× победитель): четыре критерия по порядку — проблема для конкретного человека в одном предложении (первые 30 секунд), работающее демо с вводом «real input typed in front of them», осознанные trade-offs («что вырезали и почему»), техническая достоверность ответов; веса его рубрики: problem 25, demo 30, scope 20, pitch 15, feasibility 10. [dev.to](https://dev.to/pranjulrathour/what-hackathon-judges-actually-look-for-a-rubric-from-both-sides-of-the-table-39o)
- Typичные ошибки: over-indexing на оригинальность при сырой демке; «tech for tech's sake». [Eventornado](https://eventornado.com/blog/how-to-judge-a-hackathon-5-criteria-to-pick-winners), [TAIKAI](https://taikai.network/en/blog/hackathon-judging)

**Выводы**
- Для двусторонней платформы главный риск — показать обе стороны за 5 минут без потери нити. Решает один сквозной пример, введённый вживую, и явный список того, что вырезано (/teams, чат, регистрация — ТЗ §7 это разрешает).
- ТЗ §11 требует ввод слабого описания на защите: это совпадает с «real input typed».

**Применить у нас**
- Демо начинать с ввода 1–2 строк живьём, не с seed-карточки. Seed-карточки — только для каталога и второго отклика.
- В README и в конце демо — блок «Сознательно не делали» с ссылкой на ТЗ §7.
- Репетировать отказ AI: показать, что при ошибке API включается mock и сценарий не ломается (жюри это ценит как «composure during technical failures»).

## Топ-5 применить сегодня

1. Подсказки missing: «что дописать» + «+N баллов» + пример ответа; подпись «готовность, не оценка компании»; «до уровня "готовая" не хватает N».
2. Приоритет уточняющих вопросов: данные → критерии успеха → контакт/формат; признаки полноты для data (срок/способ передачи) и interaction_format (периодичность).
3. Nullable-поля в схеме Card + страница `/ai` с промптом, схемой и живым примером срезанной выдумки.
4. Демо: живой ввод слабого черновика первым действием; блок «сознательно не делали» со ссылкой на ТЗ §7.
5. Язык заказчика: «отраслевые задачи», «команды AI-Sana», «акселерация»; отрасли seed — из приоритетов программы.

## Не делать

- Не давать баллы за длину текста без признака (Goodhart; LinkedIn-критика «completeness, not quality»).
- Не скрывать низкорейтинговые задачи и не блокировать отклик (ТЗ §4 прямо запрещает; аналоги тоже не скрывают).
- Не строить рекомендации/автоподбор до 16:00: у судей это «tech for tech's sake», а ТЗ §5 запрещает назначение.
- Не называть продукт «маркетплейс» в питче для МНВО, говорить «хаб задач для команд AI-Sana».
