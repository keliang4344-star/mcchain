<div align="center">

# MobileChain Philosophy Whitepaper

### An Epic of "The Return of Ownership"

**A Public Chain That Puts a Full Node in Every Phone**

Chain ID `mcchain-mainnet-1` · Native coin MC (smallest unit umc, precision 6) · Fixed supply 1 billion · Zero inflation

</div>

---

> **The Three Unwritten Conventions of This Whitepaper**
>
> One. This document tells only of our own road. Every key figure — supply, distribution ratios, slashing rules, attestation validity — corresponds to a real constant on a line of source code, which anyone can read on-chain and verify in the code. We do not write "vision numbers", only "code numbers".
>
> Two. Any capability not yet implemented on mainnet is marked "in planning"; we never write an intention up as an accomplished fact.
>
> Three. Wherever this document and the code disagree, the code prevails. This document is an explanation of the code, not a promise about it.
>
> Chapter Thirty-Two lists the known limits of this design and of the present launch state. It is part of the argument, not a disclaimer appended after the argument — the reader who skips it has not finished reading this document.

---

# Prologue. An Unfinished History of the Return

Humanity has two histories. One is written in the chronicles, and tells of wars, dynasties and heroes; the other is written in the ledger, and tells of who owns what, who produces value, and who takes the returns. The second is rarely spoken of, yet it is the more lethal — because it decides whether an ordinary person is the master of his own labour, or a priced line in someone else's ledger.

This history of the ledger has one motif running through all of it: **every iteration of technology has redistributed some capability once monopolised by the few back to the many.** Writing carried knowledge out of the priest's inner chamber; printing carried books out of the monastery; the steam engine freed human muscle from repetitive toil; electricity brought motive power into every household; the internet pressed the marginal cost of information down to almost nothing. Each time, technology did the same thing — it lowered the threshold of "participation" and raised the cost of "monopoly".

Yet when the digital age arrived, this history developed a crack.

We invented the internet and delivered information to every hand for free; we invented the smartphone and put a networked, computing machine into every pocket; we invented artificial intelligence and, for the first time, drew the scarcest thing of all — "intelligence" — out of the human brain and turned it into a service that can be called and priced. And yet, when all of this happened, those who truly took the value were not the people who lit their phone screens late at night, produced data at their fingertips, and supplied compute from their pockets. The value flowed to a handful of platforms, a handful of machine rooms, a handful of companies holding the servers and the sovereignty over data.

This is an absurd and entirely real paradox: for the first time in human history, ordinary people hold the means of production in their hands all at once — the phone, the data, the idle compute — and are yet structurally excluded from ownership.

MobileChain wants to supply the step that this crack is missing. It does not claim to have invented much that is new; it wants only to do one thing that is far too long overdue — **to make the person whose screen glows in a pocket once again the master of what he produces; to make the network woven from billions of phones truly belong to those billions of people.**

This is the story this whitepaper sets out to tell. It does not begin with code; it begins with a longer history of the ledger. For to understand why MobileChain exists, you must first understand this: the return of ownership has always been the foot that carries human civilisation forward.

---

# Volume One. The Long River: The Repeated Transfer of Economic Power

*This volume does not discuss a single line of code. It discusses MobileChain's coordinates within the whole of human economic history — why its arrival is not a technical speculation, but the necessity of a technical iteration.*

---

## Chapter One. The Vessel of Value: A Brief History of Human Economic Power

To understand today, pull the lens back five thousand years.

The way humanity organises value has passed through several total changes in the nature of its vessel. Each change was not a smooth improvement but an earthquake of power — the old vessel locked value in the hands of the few, the new vessel let value flow to more people.

**The first vessel was physical goods.** Shells, grain and livestock all once served as the prototypes of money. What they had in common: value was bound to matter, hard to divide and hard to move over distance. In a society that prices things in cattle, wealth gathers by nature in the hands of those who can raise cattle.

**The second vessel was metal.** Around 600 BC, the Lydians of Asia Minor began to mint standardised coins of gold and silver. Metal money abstracted "value" out of concrete matter for the first time, making it divisible, storable and transportable across regions. It gave birth to the first true "markets" and the first "merchant class" in history — power passed, in part, from the landowners to the traders.

**The third vessel was paper.** The jiaozi of Northern Song China (around AD 1024) was the world's earliest paper money, and behind it "credit" replaced "matter" as the bearer of value for the first time. Paper money cut the cost of moving value down to the weight of a sheet of paper, and for the first time made the "right of issue" one of the state's most essential powers. Whoever prints the notes holds the tap of wealth.

**The fourth vessel was bank credit.** By the twentieth century, money had broken entirely free of its metal anchor — in 1971 the Bretton Woods system collapsed, the dollar came off gold, and humanity entered the age of pure credit money. The overwhelming majority of the "money" in your account today is nothing but a line of digits in a bank's database, conjured out of nothing by a commercial bank through a loan. The vessel of value moved out of the vault and into the centralised database.

**The fifth vessel is being born.** The blockchain raised a startling question: if the recording and transfer of value need no longer depend on any central database, but on a public ledger anyone can verify, then can the "right of issue" and the "right to keep the books" also be returned from institutions to the network itself? Bitcoin answered that "bookkeeping" can be decentralised; what MobileChain wants to answer is the more concrete and more ordinary half — **can the right to participate and the right to the returns also be decentralised?**

The succession of these five vessels reveals a rule: **whenever the vessel of value grows smaller, lighter and cheaper, the old structure of power is pried loose once more.** Metal pried at the land, paper pried at the vault, credit pried at gold, and what the blockchain means to pry at is the central database. But mark the second layer of the rule — after the vessel changes hands, power tends to disperse briefly and is then recaptured by a new centre: paper money gave rise to central-bank centralisation, credit money gave rise to commercial-bank centralisation, the internet gave rise to platform centralisation. MobileChain stands at the very end of this thread, and what it means to do is not merely to pry, but to **use the distribution rules written hard into the code to lock the power released this time permanently in the hands of the participants, so that it is not quietly absorbed by the next centre** — to hand this vessel back, one last time, to the hand that holds the phone. This is what most fundamentally sets it apart from the five vessel-changes before it, and it is the ground on which it dares to call itself "the return of ownership" rather than "yet another transfer".

---

## Chapter Two. The Paradox of the Platform Age: We Created the Data, Yet Lost the Ownership

In the last two decades of the twentieth century, humanity passed through a silent revolution. The internet sent information out for free, the smartphone put a networked computer into every pocket, and Web 2.0 handed the ability to "produce content" to every user.

By the script of history, this should have been another moment when ordinary people took back power — just as printing taught commoners to read and the internet gave commoners a voice. But this time the script went astray.

The platforms appeared. They did one thing, shrewd and hidden: **they took the user's output and gathered it back into the platform's own possession.** Every post you write, every video you shoot, every click you leave behind, every record in your phone of where you went and what you did, becomes raw material for the platform to train its algorithms, place its ads and raise its valuation. What you get in return is a "free" right of use, and a terms-of-service agreement you will never finish reading and will never be able to change.

This is a new kind of owner—producer separation. In the agrarian age it appeared as the farmer and the land; in the industrial age, as the worker and the machine; in the digital age, it appears as the user and the data. The only difference is this: the first two separations were visible (the deed, the factory), while this separation is invisible — your data, which you can neither see, nor touch, nor take a share of the returns from.

The deeper paradox is this: the most important "means of production" of this age is no longer land or machines, but **data, and the compute that processes data**. And these two things lie precisely in the pocket of every ordinary person — your phone is generating data at every moment, and leaving its compute idle at every moment. Yet these "means of production" are requisitioned without payment by the platforms in the name of "free service", and then sold back to you at double the price in the form of "advertising" and "subscriptions".

This is the fundamental injustice of the platform age: **the means of production and the labourer have never been so close, and the distance of ownership has never been so far.** A farmer at least knows where the land he tills is; a user does not know where the data he produces has gone, what it is worth, or who has earned from it.

This pattern of "requisition—withholding" is almost written into the genes of every giant platform, and a few examples that everyone has lived through make it plain: ride-hailing platforms turn the driving traces and supply-demand heat maps of millions of drivers into their own dispatch algorithms and valuation, while the driver is merely a temp paid by the trip, sharing nothing of the platform's rise in value; short-video platforms train the creations, dwell times and interactions of hundreds of millions of users into a recommendation engine, and what the creator receives is a sliver of traffic allotted by the algorithm, while the platform receives the wealth of advertising and a public listing; map and navigation apps turn users' real-time locations and traffic contributions into the most precise traffic database in existence, and what the user receives is "free navigation". Every time, the user is an unpaid "data tenant" and the platform is a "digital lord" collecting rent without lifting a hand.

A subtler and more hidden layer: **the platform buys out your bargaining power with "free".** Because you did not pay for the service, you have no ground to claim ownership; because your data is collected without payment in the name of "terms of use", you cannot even ask what you have been paid. This is an ingeniously designed evaporation of ownership — not a seizure by force, but a sugar coating of "free" that makes you hand over the means of production of your own will, and be grateful besides.

This is why MobileChain must break through head-on. What it sets out to answer is precisely this question that has been set aside for twenty years. It does not rely on persuading platforms to be good — goodness is unreliable and unverifiable. It relies on writing "output is priced, and priced output is rewarded" into the code, so that the producer of data can, for the first time, take back what is his own in technical fact, and not only in moral claim.

---

## Chapter Three. The Inevitability of Technological Iteration: Every Cost Reduction Triggers a Redistribution of Power

Lift your gaze a little higher, and you will find a regularity that is almost like a law of physics:

> **Every technology that presses the cost of producing or acquiring some key resource down by an order of magnitude will, in the end, trigger a redistribution of power.**

The evidence is everywhere you look:

- **Printing (Gutenberg, c. 1440).** Before it, copying out a single book took months, and knowledge was monopolised by the church and the nobility. Printing pressed the cost of reproducing a book down by hundreds, even thousands, of times. The result was not simply that "books became cheaper" — it set off the Reformation, the Scientific Revolution, the unification of national tongues. The ownership of knowledge spread from the few to the literate many. Power moved with it.
- **The steam engine (Watt's improvements, 1769).** It freed "motive power" from the geographically bounded resources of water and animal muscle and carried it into any factory at all. The result: production no longer depended on land and rivers; capital and the factory owner replaced the landlord as the new centre of power. This was a redistribution from "ownership of land" to "ownership of capital."
- **Electricity (late 19th century).** It turned motive power into a general-purpose commodity that could be sent long distances over a grid, letting factories be sited anywhere and household appliances enter the home. The result: productive capacity spread further, the middle class grew, and the consumer society took shape.
- **The internet (ARPANET in 1969, the World Wide Web in 1991).** It pressed the marginal cost of "transmitting information" down towards zero. The result: the right to disseminate information shifted partly away from newspapers and television stations to anyone who could post. A redistribution from "ownership of the media" to "the right of expression."

Note the other half of this regularity: **a resource whose cost has been crushed will in the end be recaptured by a new power structure, unless an institution locks it into the hands of the many.** The internet drove the cost of information to zero, but platforms, through the model of "free service in exchange for data," concentrated informational power all over again. This is history's usual script — technology loosens the bonds, power spreads briefly, and then a new centre absorbs it back.

What MobileChain is betting on is breaking that second half of the script. What it sets out to do, in essence, is to press the cost of "participating in a public chain, contributing real compute and data, and being rewarded accordingly" down from "a mining rig, a server, a million in stake" to "a phone." By that law, when the cost of participation drops by an order of magnitude, power should be redistributed once more — and this time, MobileChain, through allocation rules written hard into code, tries to lock power **permanently, and beyond any quiet recapture,** into the hands of participants.

This is not idealism. It is one more application of the law of technological iteration — only this time, the object is the phone in your pocket.

---

### Further Evidence: How Cost-Crushed Technologies Redrew the Map of Power, Again and Again

That regularity stated above — "a technology that presses the cost of a key resource down by an order of magnitude will in the end trigger a redistribution of power" — is not an isolated claim resting on four or five cases. Widen the lens and you will see it, like a hidden thread running through modern history, playing out again and again. For every further case, MobileChain's arrival takes on another shade of "inevitability."

**The telegraph and the telephone (mid-to-late 19th century).** Before the telegraph, long-distance communication was expensive enough that only states and great trading houses could afford it, and informational power was concentrated in the postal and diplomatic apparatus. The telegraph cut transcontinental transmission from weeks to seconds, and the telephone made it an everyday thing for ordinary people. The result: commercial decision-making sank from head office down to the local agent; the news media turned from "relaying the official line" into "independent reporting"; even the shape of war changed — for the first time, command could extend to the front in real time. The collapse in the cost of communication redrew the map of informational power.

**The shipping container (McLean's first standardised voyage, 1956).** It looks like an unremarkable steel box, yet it may be the most underrated power-shifting device of the 20th century. Before the container, cargo was loaded and unloaded by the shoulders and hands of hundreds of dockworkers, and the bottleneck of trade lay in the ports, in manpower, in the unions. The container standardised and mechanised loading and unloading, cut a single ship's turnaround from weeks to hours, and sent unit logistics costs plummeting by more than ninety percent. The result: the global supply chain took shape in earnest for the first time, and manufacturing power moved on a vast scale from Europe and America to lower-wage East Asia — a redistribution from "the barrier of manpower" to "capital and location," which laid the ground directly for the later shape of the "workshop of the world."

**GPS (first satellite in 1978, fully opened to civilian use in 2000).** It began as a purely military asset, its positioning accuracy deliberately degraded (the SA policy), so that civilian users could only receive a watered-down signal. In 2000 the United States switched SA off and opened high-precision positioning to all humanity, free of charge. That single act turned "knowing your precise position on Earth" from a military privilege into the default right of every phone user. Today's food delivery, ride-hailing, logistics and autonomous farm machinery all rest on this "freely released" capability. A textbook case of "a centre voluntarily letting go of its monopoly" — the motive may not have been pure, but the outcome confirms the law: when a key capability approaches zero cost, power will in the end spill outward.

**Banking deregulation and electronic clearing (late 20th century).** Credit-card networks and electronic clearing turned a "transfer" from a physical errand that required a trip to the counter into a set of signals travelling down a telephone line. It pressed the marginal cost of financial services extremely low, allowing banks to serve ordinary savers and small merchants who had previously been overlooked — the threshold of financial power sank, for the first time, from "the large depositor" to "anyone." The price was that financial power concentrated further into a handful of clearing networks, which in turn illustrates the second half of the law: once technology loosens the bonds, power spreads briefly, then is recaptured by a new centre, until the next round of technological iteration.

**The World Wide Web (1991) and the smartphone (the iPhone, 2007).** The former liberated "publishing" from the hands of newspapers and television stations and gave it to anyone who could write HTML; the latter pushed the "networked computer" off the desk and into every pocket. Stacked together, they produced what Chapter One called "users becoming content producers" — and yet, as Chapter Two related, they were recaptured by platforms through "free service in exchange for data." This is the second half of the law playing out once more, and it is precisely the link that MobileChain sets out to break.

Set these seven or eight cases side by side and a clear line emerges: **printing liberated knowledge, the steam engine liberated motive power, electricity liberated energy, the telegraph and the telephone liberated communication, the container liberated logistics, GPS liberated position, the internet liberated information, the smartphone liberated the terminal — each time, some key capability was handed from the few to the many; and after each time, the old power structure was prised open, and a new centre tried to absorb it back.** MobileChain stands at the far end of this line, and what it means to hand over is the last and the nearest of them all: the capacity to "participate in a network of value and, on that basis, hold ownership of it" — from mining rigs, servers and a million in stake, to a single phone. History is not yet finished here, and this time the rules of allocation are written into the code.

---

## Chapter Four. The Missing Step: Why "The Value of a Network Belongs to Its Participants" Has Not Happened to This Day

Economists have long been clear about one thing: the value of a network comes from its participants. Metcalfe's Law says that the value of a network is proportional to the square of its users. A social network is worth a fortune not because it has handsome servers, but because billions of people produce content and form connections on it every day.

And yet the distribution of value across almost every large network runs counter to this law: **those who create the value (the users) do not receive it; those who take the value (the shareholders, the platforms) are often not its creators.** Every post you publish lifts this company's valuation, and your reward is a free service — and advertising sold to you with growing precision and at a growing price.

Why is it that "the value of a network belongs to its participants," a slogan cried out for so many years, has still never truly come to pass? Three structural reasons:

**First, the threshold of the means of production.** On the traditional internet, when you "participate" in a network you merely contribute data; you do not "own" a fragment of the network. To truly own it you would have to be an employee, a shareholder, a node operator — and each of these has a threshold. The ordinary user is forever kept outside the door of the "owner" and left to be a mere "contributor."

**Second, the centralisation of the right to keep the books.** Even if someone wanted a fair distribution, a traditional centralised database cannot supply a trustworthy, tamper-proof record of it. The platform says who gets what and how much; it sets the rules and it keeps the accounts, and you have no way to verify them. Trust falls back, once more, on dependence on the platform.

**Third, the paradox of scale and fairness.** A small community can distribute by personal favour, but a network of billions that tried to do so would inevitably collapse. To distribute fairly among billions, there must be a set of automatic, trustworthy rules that no one can quietly alter — and before the blockchain, no such rules existed.

These three points map exactly onto the three things the blockchain can solve: lowering the threshold of the means of production with a phone, providing tamper-proof record-keeping with a public ledger, and achieving large-scale automatic distribution with rules in code. That is to say, **"the value of a network belongs to its participants" has been so long in coming not because it ought not to happen, but because the technical conditions only became complete for the first time once the blockchain matured.**

MobileChain's arrival rests on this judgement: the conditions are already ripe, and all that is missing is a chain that treats "a phone" as the means of production and "real contribution" as the basis of distribution. It does not conjure a concept out of thin air; it gathers technical conditions that are already ripe and converges them, for the first time, onto "the phone in your pocket."

---

## Chapter Five. The Silent Majority in Our Pockets: Five Billion Devices, the Forgotten Means of Production

Let us bring the camera down from the five-thousand-year river and focus it on the phone in your pocket at this very moment.

There are more than five billion smartphones on Earth. That is a larger number than any "means of production" in history—it exceeds the world's total automobiles, exceeds the total number of industrial machines, and even exceeds the number of people who have received a higher education. Each of these five billion devices contains:

- a chip capable of running neural-network inference;
- a continuous supply of grid power and a wireless connection;
- a sensor array that can be read at any moment (positioning, microphone, camera, motion);
- and a real human user, who spends hours with it every day.

Yet for 99% of the time, the computing power of these devices idles, their data is collected without payment, and their connections are consumed for nothing. They are the most widely distributed, and yet the most underestimated, "silent means of production" in human history.

Let us do a rough reckoning (only to illustrate the order of magnitude, not as a precise prediction): if even one-tenth of them—five hundred million phones—contributed a portion of their idle compute and online time to a decentralised network, the resulting total compute would far exceed the marginal redundancy of any single cloud provider; the data annotation, inference, and verification tasks they generated would be enough to sustain a genuine edge-AI market; and their character—distributed across the globe, dependent on no central data centre—is precisely the resilience that a centralised cloud can never replicate.

The problem was never "this compute does not exist," but "it has never been priced, never been rewarded, never been owned by its owner." It is quietly requisitioned by platforms in exchange for free services, yet never entered onto any balance sheet belonging to the user.

What MobileChain sets out to do is precisely to issue these five billion silent devices a balance sheet of their own. When a phone, for the first time, receives a reward—written into a public ledger, belonging to its owner—for a task it truly completed, an online presence it truly provided, data it truly produced, that device becomes for the first time not "a requisitioned means of production" but "an autonomous producer."

This is the largest re-confirmation of the means of production in economic history; only its vehicle is no longer land or machines, but the faintly warm pane of glass in everyone's pocket.

To explain this fully requires a little economics. Traditional economics has always measured "factors of production" as land, labour, and capital. In the digital age, more and more economists acknowledge that a fourth is missing: **data, and the compute that processes data**. Every swipe you make, every location you fix, every time you speak to a voice assistant, you are producing data; the chip in your phone is supplying compute. These two are precisely the scarcest inputs of the modern AI economy.

The paradox is this: under the platform model, this fourth factor of production is requisitioned for free. You supply the data; the platform uses the data to train models and lift its valuation. You supply idle compute and online presence; the platform uses your activity to prop up its network effects. In return you receive "free" services—services whose cost has long since been collected back, doubled, through advertising and subscriptions. This is a hidden "data labour": you have laboured, yet you are not recognised as a labourer; you have produced, yet you are entered onto no balance sheet that is yours.

What MobileChain seeks to correct is precisely this missing price on the fourth factor of production. It does not make moral appeals; it makes technical arrangements: it turns acts such as "providing online presence," "completing inference," and "generating verifiable data" into real contributions that are measurable, priceable, and rewardable on-chain. When the idle compute of a phone is, for the first time, written—because of a task it truly completed—into a ledger belonging to its owner, the fourth factor of production acquires, for the first time, a price that belongs to the producer.

Going further, this is the repricing of a long-undervalued "idle asset." Economics holds a plain truth: an asset that is neither used nor priced has a social value of zero, even if it truly exists. The idle compute of five billion phones worldwide is exactly such a piece of social wealth—"existing, yet zeroed." What MobileChain does is, in essence, to build price discovery for this wealth—not through some central pricing, but through buyers and sellers transacting spontaneously on-chain. The demand side bids, the supply side takes the order, and the price is formed by the market. When this wealth is priced, traded, and rewarded, it turns from "the silent means of production" into "an active factor of production," and the total compute supply of society gains an entire order of magnitude out of thin air.

This is the precise economic meaning of "All under Heaven belongs to all under Heaven": the largest and most underestimated means of production belongs not to any one company, but to the billions of people holding phones; and its returns, too, should flow back along the path of true contribution to those billions, rather than pouring into a handful of data centres.

---

## Chapter Six. "All under Heaven Belongs to All under Heaven": A Political Philosophy of Ownership

Here is an ancient political proposition, worth setting down with due gravity in this document:

> **All under Heaven ought always to belong to all under Heaven.**

The meaning of this sentence, translated into the language of modern political economy, is: **an infrastructure that is jointly participated in, jointly built, and jointly contributed to by countless people must, in the end, return its ownership and its right to returns to those participants themselves, rather than being permanently intercepted by a tiny centre.**

This is no idle fancy. It is a direction that human political history has repeatedly verified. In the feudal age, land and power belonged to the sovereign; the bourgeois revolutions returned power in part to the propertied; universal suffrage and constitutionalism gradually returned political rights to a broader populace. Each time, "All under Heaven" moved one step closer to "all under Heaven's people"—slowly, and incompletely, but it moved.

But in the digital age, this step has gone into reverse. We have created networks built jointly by all their users, the like of which never existed before, and then locked the property rights to those networks inside the shares of a few listed companies. Billions of people build them daily, use them, contribute data to them, and yet own not a cent of them. This is a new form of "digital feudalism"—the lord replaced by the platform, the serf replaced by the user, the land rent replaced by data and attention.

MobileChain's political position stands on the negation of this layer of "digital feudalism." It holds that:

- the value of a network jointly constituted by billions of phones should belong not to the shareholders of some company, but to its true contributors;
- every "point" that contributes online presence, compute, and data to the ecosystem is one of the genuine owners of this network, and deserves rights and returns commensurate with its contribution;
- such rights and returns cannot be entrusted to the goodwill of an operator, but must be written into code that no one can secretly alter, jointly verified and jointly guarded by all participants.

This is the deeper reason MobileChain treats "open-source and auditable, parameters written into code" as its very lifeblood—it is not merely a technical principle but a political bottom line: **only when no one can stand above the rules does ownership truly return to all under Heaven.**

At this point, the argument of Volume One closes: seen historically, the return of ownership is the march of civilisation; seen politically, "all under Heaven returning to all its people" is a digital revolution yet unfinished; seen economically, five billion phones are a forgotten means of production. The three converge on a single conclusion—**it is time for participants to become, once again, the owners of what they produce.** And MobileChain is an engineering practice aimed straight at that conclusion.

---

# Volume Two. The Return: How MobileChain Completes This Transfer

*This volume discusses technology, but every section returns to the conclusion of Volume One: every mechanism of MobileChain exists to make ownership and the right to returns flow back, from the centre, into the hand that holds the phone.*

---

## Chapter Seven. One Phone, One Node: Handing Participation Back to the Pocket

Volume One said that the injustice of the platform age has its root in the "threshold of the means of production." MobileChain's first cut is to chop that threshold away.

Running a full node on a phone means that "becoming a real part of a public chain" no longer requires mining rigs, servers, or a million in staking—only an ordinary phone that has passed attestation. This is a drop of an order of magnitude in the cost of participation—and by the law stated in Chapter Three, when the cost of participation falls by an order of magnitude, power must be redistributed once more. What MobileChain wants is precisely this redistribution.

But a phone is not a server. It goes offline, it can be counterfeited, its compute is limited. The `phonenode` module does not pretend that a phone is a server; instead it **accepts the nature of the phone, and designs its security rules around that nature**:

- **Attestation is required and valid for 30 days**—without valid attestation you cannot participate in device incentives; once it expires you must re-attest, which both blocks bare emulators and squeezes the window for counterfeit returns.
- **Sybil device binding is enabled**—one real device identity corresponds to one unit of incentive eligibility, driving the cost of counterfeiting "one server pretending to be ten thousand phones" up exponentially.
- **Offline grace and tiered slashing**—going offline is the lightest (5%), cheating on contributions is heavier (10%), forging attestation is the heaviest (20%); being offline is no crime, attacking the foundation of trust is.

| Parameter | Value | Meaning |
|---|---|---|
| `AttestationValidity` | 2,592,000 seconds (30 days) | attestation validity |
| `OfflineGraceBlocks` | 100 blocks | offline grace window |
| `OfflineSlashBps` | 500 (5%) | slashing for going offline |
| `ContribSlashBps` | 1000 (10%) | slashing for cheating on contributions |
| `AttestSlashBps` | 2000 (20%) | slashing for forging attestation |
| `SlashCooldownBlocks` | 43200 blocks (about 12 hours) | cooldown before re-attestation after slashing |

**A boundary that must be made clear:** phone nodes participate in device incentives (DePIN), and their threshold is very low; validators participate in consensus block production, and require at least 30,000 MC of self-bonded stake, a high threshold and a heavy responsibility. MobileChain is extremely tolerant of "the breadth of participation" and extremely strict about "the responsibility for security." Breadth rests on numbers of people, security rests on weight of responsibility; two sets of thresholds, each safeguarding its own.

This step is the most concrete engineering landing point of "all under Heaven returning to all its people": **to let ordinary people who do not have 30,000 MC also participate truly, through a single phone, and receive returns from it.** When a phone, on a nightstand in the small hours, quietly produces a block, completes an AI inference, and collects a modest sum of MC, the belated return of ownership from Volume One takes place, truly, by one inch, in this ordinary person.

---

## Chapter Eight. Scarcity Written into Code: From Scarcity by Promise to Scarcity Locked by Fact

Volume One recounted how the vessel of value moved out of the vault and into the central database, and how "the right to issue" became the core power. What MobileChain does is hand over this last remnant of central power as well.

The supply model in three sentences:

1. **Total supply = 1 billion MC** (`1,000,000,000,000,000 umc`, precision 6);
2. **Minted in full once at genesis**, after which the block reward is forever 0 and the `x/mint` inflation rate is forced to 0;
3. **Never any further issuance** — the ceiling is a constant hard-wired into the code, `TotalSupplyCap = uint64(1e15)`, and it does not enter the governable parameter space. To change it, you would have to change the source, recompile, and persuade the entire validator set to hard-fork.

```
// x/tokenomics/types/keys.go
DefaultDenom   = "umc"
TotalSupplyCap = uint64(1e15)   // = 1 billion MC, an on-chain constant with genesis verification as a second lock; not governance-changeable
```

A political stance hides here: **scarcity should not be a promise that can be abandoned at any moment, but a fact that no one can abandon.** "The team promises not to issue more" depends on goodwill; "the supply ceiling is an on-chain constant that governance cannot alter" depends on mathematics. MobileChain replaces trust in "people" with trust in "code" — because history has proved again and again that whoever holds the right to issue will, sooner or later, be unable to resist turning on that tap.

Why 1 billion? Too small (on the order of millions) and everyday micro-incentives cannot be expressed in whole numbers; too large (on the order of trillions) and it whispers "print at will." 1 billion with precision 6 is the balance point between engineering usability and the psychological sense of scarcity.

And beyond a fixed supply, MobileChain also builds in permanent deflation, drawn only from two sources — "protocol usage fees" and "slashing for misbehaviour" — and never touching the labour income of participants: 7% of gas fees burned each period, half of all DEX trading fees burned permanently (equivalent to 0.15% of traded volume), and 40% of validator slashing burned permanently — all sent to a black-hole address with no corresponding private key. The three classes of participant earnings — device task bounties, referral rewards, and EdgeAI task settlement — are subject to no burn deduction on-chain and arrive 100% in full. Burn = shrinking supply = benefit to holders = more nodes participating = more tasks = more burns, a positive-feedback flywheel. Scarcity, therefore, is not merely "no further issuance" but "continuously reinforced through real use."

---

## Chapter Nine. The Five Pools as Contract: The Largest Share Goes to the Real Builders

At genesis the 1 billion MC is allocated once across the Five Pools. These are not arbitrary proportions; they are the final version derived directly from the positioning of "phone full node, fixed supply":

| Pool | Share | Amount (MC) | Custody | Release Cadence |
|---|---|---|---|---|
| **Device Incentive DePIN** | **55%** | 550 million | depin module account | Paid out task by task, against real tasks |
| **Staking / Network Security** | **15%** | 150 million | staking_security module account | Governance drip-feeds on schedule |
| **Team** | **12%** | 120 million | 3-of-5 multisig vesting | 1-year cliff + 3-year linear |
| **Foundation** | **13%** | 130 million | Custodian address overridden by explicit public key (multisig/cold wallet; takes effect once the placeholder is replaced) | 50 million at once at T0 (including 5 million into the DEX initial pool) + 80 million linear over 2 years |
| **Early Development** | **5%** | 50 million | Custodian address overridden by explicit public key (same as above) | Released for substantive contribution before and after mainnet |

```
DeviceIncentivePercentBps = uint32(5500)   // Device Incentive 55%
StakingSecurityPercentBps = uint32(1500)   // Staking Security 15%
TeamPercentBps            = uint32(1200)   // Team 12%
FoundationPercentBps      = uint32(1300)   // Foundation 13%
EarlyDevPercentBps        = uint32(500)    // Early Development 5%
```

Genesis verification forces the five basis points to sum to exactly 10000 and the number of allocations to be exactly 5 — one too many, one too few, or a wrong proportion, and the chain will not start.

**The Device Incentive Pool is the only block in the entire allocation that exceeds half, and it is 3.6 times the Staking Pool.** This "share so high it is rare in the industry" is a logical necessity: MobileChain's "community" is the people who run phone nodes. A chain that sells itself on the "phone full node" cannot expect anyone to show up if a phone cannot mine a meaningful share. So the largest share must go to the real builders — and this is precisely the numerical expression of the political stance in Chapter Six: **the principal builders of the network should receive the principal share of the network.**

By contrast, the Team Pool takes only 12%, locked into a 3-of-5 multisig with a 1-year cliff plus 3-year linear release, binding the team's interests to the network's for at least 4 years. **How much a team keeps for itself is the strongest signal it sends to the community.** The restraint a system shows toward its own creators' appetites is the proof of its sincerity toward every participant.

---

## Chapter Ten. Displacing Trust: From Trust in Institutions to Trust in Mathematics

Volume One pointed out that when we entrust our data to institutions in the platform age, we are in essence entrusting our trust to "people." MobileChain systematically displaces that trust with trust in "code":

- The Device Incentive Pool has no minting authority; it can only pay out of the 550 million already in existence, each payment leaving one fewer, openly visible and verifiable;
- The Foundation and Early Development shares are held in custodian addresses overridden by explicit public keys (multisig/cold wallet), and release is executed by code according to an on-chain timetable; the governance goal is that "every drawdown passes through an on-chain proposal and a community vote," and the machine-enforced execution of that constraint (timelock/governance treasury) is on the roadmap, as honestly disclosed in Chapter Thirty-Two, Section One;
- EdgeAI arbitration is carried by a multisig at deployment and can become governance-driven in the future;
- The validator minimum self-bond (30,000 MC) is enforced at the chain level by `app/ante.go`, and no one can bypass it.

> **A healthy system should not ask participants to believe in the goodwill of some person or some team; it should make the rules themselves worthy of trust.**

This is not a distrust of people, but this: **a system that does not depend on trusting any particular individual is the only kind that can truly endure.** When trust is displaced from "some name" onto "a piece of code anyone can read," power completes its last and quietest return — not to another centre, but to a public ledger that everyone can verify.

---

## Chapter Eleven. Resonance Distribution: Keeping Incentives in Step with Real Contribution

Many projects are zero-sum games at their core: one person's gain comes from another's loss, and the system creates no real value. MobileChain rejects that path and names its distribution philosophy **Resonance Distribution** — the rhythm of incentives should "resonate" with the rhythm of the network's real contribution, not with the mechanical passage of time.

Concretely:

- **Paid per real task** — triggered only when a verifiable task is actually completed (proof of being online, edge AI inference, and the like);
- **Paid only after attestation** — checking attestation validity, sybil binding, and slashing cooldown;
- **Linear vesting to avoid sell pressure** — released linearly over several years, rather than dumped onto the market all at once.

The core of "resonance": **the more active the network and the more real contribution there is, the more fully incentives are released; when things are quiet and there is no contribution, the vault automatically slows its own depletion.** Incentives stay in step with real value, not with speculation.

Anti-farming is the lifeline. The 550 million MC is guarded by seven layers of defence: identity threshold, unique binding, attestation validity, task proof, tiered slashing, slashing cooldown, oracle verification. Any farming script would have to break through all seven at once, each layer more expensive than the last, exponentially so. **A low threshold for real users, a high cost for cheaters** — this is the confidence that lets MobileChain hand 55% to device incentives.

A single MC in circulation should correspond to some real-world contribution — the uptime of a phone, the compute of one AI inference, the value of a real transaction. This is the engineering of the thesis in Chapter Six: your output is priced; your price is rewarded; your reward is written into the ledger and belongs to you.

---

## Chapter Twelve. Edge Compute: Returning the Means of Production of the AI Age to Its Users

If DePIN makes a phone's "being online" valuable, EdgeAI makes a phone's "computing" valuable — and this is the sharpest cut MobileChain makes on the proposition of "returning the means of production."

Two trends unfold at once: demand for AI inference grows exponentially, while compute supply is highly monopolised by a handful of cloud vendors — expensive, gated, and centralised. At the same time, the compute power of billions of phones sits idle most of the time. The idea behind EdgeAI: **organise scattered phone compute into a decentralised edge AI compute market** — those with compute sell their idle capacity to those who need it, and the two sides settle on-chain, verifiably and with recourse to arbitration.

It uses a **payer-funded escrow model plus optimistic settlement**: the payer locks the bounty into on-chain escrow when posting a task; the compute provider commits to the computation only after seeing the money locked; once the result passes and the dispute window elapses, the bounty is released automatically. The money is locked first, and the code releases it once the work is done — neither side has to trust the other, only the escrow contract.

Each settled EdgeAI task bounty is split 85% (executing node) / 15% (reserved for verifier nodes), with no burn deduction on-chain — the bounty escrowed by the payer flows in full to those who compute and to those who verify execution; disputes rest on optimistic settlement — the result is presumed honest by default unless someone objects within a 100-block window. The great majority of honest tasks take the fast lane of "zero dispute, automatic settlement," consuming no arbitration resources.

The significance of EdgeAI is that it brings the argument of Volume One, Chapter Five down into commercial reality: **those 5 billion forgotten means of production can, for the first time, technically be priced, be rewarded, and be owned by their owners.** Once payer-funded fees reach scale, phone-node income no longer depends only on the 550 million pre-minted vault, but acquires a cash flow from real AI compute demand — this is the key bridge from "pre-minted" to "real income" for device incentives, and the most concrete economic proof of "returning the means of production to their users."

Seen politically, EdgeAI touches a sharper proposition still: **compute sovereignty.** Today, the AI inference of a country, a company, or an ordinary person must often travel thousands of miles back to a handful of data centres located in particular jurisdictions, be computed by the GPUs there, and only then be sent back. Your data, the moment it is created, leaves your jurisdiction and leaves your boundary of control. This is an invisible surrender of sovereignty — every day you outsource your most sensitive "intelligent activity" to servers thousands of kilometres away that belong to someone else.

MobileChain's edge compute model offers another possibility: inference is completed on your phone, in your jurisdiction, under your physical control; the raw data need not leave the border, and only the verifiable result is settled on-chain. For an actor that values data sovereignty and is unwilling to entrust the intelligent activity of its citizens wholesale to overseas data centres, this edge compute network made of domestic users' phones is itself a sovereign asset at the level of infrastructure. It scatters "intelligence" back out of the monopoly of a few data centres and into the palm of every user and the boundary of every community. This layer of meaning goes beyond commerce and enters the territory of politics — **when compute is no longer monopolised by a few data centres, the means of production of the AI age can, for the first time, return to users themselves, into the hands of users' own jurisdictions.** This may be MobileChain's most far-sighted footnote, in the political dimension, to "All under Heaven belongs to all under Heaven."

---

## Chapter Thirteen. Security Without Inflation: An Incentive Structure That Does No Evil

The last mechanism answers a question that seems to contradict itself: with no new issuance, what feeds the validators?

A traditional inflationary chain feeds its security by issuing new tokens, and the price is that every holder is continuously diluted — you have done nothing, and yet your share keeps shrinking. MobileChain chooses zero inflation, and it stands on three legs:

1. **The pre-minted Staking Security Pool (15% = 150 million MC)** — the main force in the launch phase, drip-released to validators by governance;
2. **Fee recirculation** — the main force in the mature phase: the net value of on-chain gas flows back to the security pool, so the more the chain is used, the thicker its security;
3. **Slashing distribution** — of the tokens slashed for wrongdoing, 40% is burned permanently (deflation across the whole network, to everyone's benefit) and 60% flows back to the Staking Security Pool, so that wrongdoing subsidises honesty.

The relay logic: the launch phase leans on the first leg, the mature phase on the second, and the third fine-tunes throughout. And so MobileChain achieves what seems contradictory — **zero inflation (diluting no one) and, over the long run, a sustained validator incentive (a network that keeps itself secure).**

A healthy dynamic: **the wrongdoer's loss becomes the honest participant's gain.** The more the network is attacked, the more its honest participants stand to gain, and attack becomes economically unsustainable. It is an echo of ancient wisdom written into code: if a community is to last, those who keep the rules must profit and those who break them must eat their own fruit — except that this time the enforcer is not the clan elder but mathematics.

---

# Volume Three. Co-Governance: The World Will Be Returned to All

*This volume is about governance. Technology releases power from the centre; governance decides whose hands it finally falls into.*

---

## Chapter Fourteen. Where Governance Ends: Above the Rules, No One Stands

The endpoint of a chain is not a successful company but a self-governing network that no longer needs any single actor. MobileChain's direction is clear: **those who created it should gradually give way to those who use it.**

Between the governable and the ungovernable runs a red line:

| Category | Example | Governance-changeable |
|---|---|---|
| **Root-of-trust parameter** | Total supply cap `TotalSupplyCap` (1 billion) | **No** — hard-written as an on-chain constant |
| **Security parameters** | Slashing basis points, attestation validity, dispute window | Yes — adjusted by governance proposal |
| **Use of funds** | Foundation Pool spending | Yes — governance proposal + vote |
| **Protocol evolution** | Module upgrades, parameter optimisation | Yes — approved by governance |

This red line: **some things must be able to change (or the chain cannot adapt to reality), and some must never change (or the promise of scarcity is empty words).** Placing the total supply cap on the "never change" side is MobileChain's hardest promise to every holder — even if governance is one day swayed by some party, it cannot quietly issue more and dilute you.

Governance is on-chain voting on a one-stake-one-weight basis, never any headcount-recruiting design. Voting power comes from your real stake in the network, giving the greatest voice to those who care most about long-term health.

When the key parameters are voted on by the community, the Foundation's funds are held by the community, arbitration is handed over to governance, and validators are sufficiently dispersed, this chain no longer belongs to a team or a foundation but to **everyone who has taken part with real devices, real compute and real contribution**. The world has never been any group's private property — it belongs to the world itself, to everyone who has spent real strength on its behalf. This is the code-made fulfilment of that line from Chapter Six: "All under Heaven belongs to all under Heaven."

Self-governance is not disorder — quite the opposite: **it is the state in which the rules are written into code and no one (including the founding team) can stand above them.** It is precisely because the core rules are fixed in code that the "order" of self-governance holds. This is also why "open-source and auditable, parameters written into code" is the political precondition of self-governance.

---

## Chapter Fifteen. Symbiosis by Consensus: Incentives Come from Real Contribution, Not Headcount

MobileChain has a community referral incentive, but it rests on one unshakeable foundation: **incentives come from real device contribution and network security, not from recruited headcount.**

Five points of difference from "recruiting headcount": no skimming (rewards are paid out additionally from the Device Incentive Pool budget); the referee's income is unchanged; there are caps (500 MC per person per day / a network-wide daily circuit breaker of 20,600 MC / a ten-generation depth ceiling, in which non-nodes enjoy only the first 5 generations); it is governance-adjustable; and it is sybil-resistant (protected by hardware attestation). The referral budget's 15% sits inside the Device Incentive's 55%, roughly 82.5 million MC in total budget, enough at full speed to support about 11 years, and actual consumption is tied to real activity.

The fundamental difference: a traditional ponzi's returns come from the principal of downline members, whereas MobileChain's referral rewards come from a pre-minted budget that has already been set aside. The referee's income is not reduced, the system's budget has a ceiling, and the community can close it by governance at any time. Symbiosis is not layer upon layer of extraction but ring within ring of mutual fulfilment, born of real contribution.

---

## Chapter Sixteen. A Faith in Transparency: Verifiable by Anyone and Only Then Truly Transparent

Transparency cannot stop at "the data is on-chain"; it must reach "anyone can look it up." MobileChain provides a Web block explorer, so that anyone can query the business data of the eight modules without running a node. Behind the explorer are each module's gRPC-gateway REST interfaces — merely one layer of visualisation over public on-chain data, which anyone can rebuild for themselves and cross-verify.

**True transparency is when anyone can verify independently, rather than relying on some official page.** When every ordinary person can check with their own hands that "the whitepaper's numbers = the code's constants", power has completed its last and quietest homecoming.

---

## Chapter Seventeen. Laying the Network Outward: A Return from Near to Far

MobileChain's network rollout follows "from near to far, from easy to hard", with Southeast Asia as the first wave:

| Region | Priority | Reason | Strategy |
|---|---|---|---|
| Southeast Asia (Vietnam / Indonesia / the Philippines / Thailand) | First wave | High phone penetration, a mature "earn with your phone" culture | Localised app + TikTok/KOL |
| South Asia (India / Bangladesh) | First wave | The largest population base, the fastest phone growth | English interface + faucet |
| Latin America (Brazil / Argentina) | Second wave | Severe inflation, strong demand for inflation-resistant assets | Spanish / Portuguese localisation |
| Africa (Nigeria / Kenya) | Second wave | The concept of phone mining has already been taught | Positioned as "a truly decentralised alternative" |
| Middle East (Türkiye / Saudi Arabia) | Third wave | A young population + high phone penetration | Arabic / Turkish localisation |
| Europe and North America | Third wave | Focus on attracting AI project teams | English-language technical community |

The multilingual cadence: mainnet launch supports Chinese and English; month 1 adds Vietnamese / Indonesian / Hindi; month 3 adds Spanish / Portuguese / Arabic; month 6 adds Japanese / Korean / Turkish / Russian.

The judgement behind it: **technological neutrality does not mean undifferentiated promotion.** An idle phone in Jakarta may create far more value than one in Silicon Valley — because the former is scarcer and more needed. MobileChain lights its first fire in the hands of those who need it most and know best how to treasure it. This is itself the geographical unfolding of "all under Heaven returning to all its people": the homecoming begins with the forgotten majority.

---

## Chapter Eighteen. The Self-Governing Community: A Network That No Longer Needs a Master

MobileChain's long-term direction: the team gradually gives way to the community. When the key parameters are voted on by the community, the Foundation is held by the community, arbitration is handed over to governance, and validators are sufficiently dispersed, true self-governance draws near.

Self-governance has three economic foundations: an asset that is scarce and undilutable (a fixed 1 billion, with no governance-permitted issuance); distribution driven by real contribution (value flowing to builders); and public funds held by the community (the 13% Foundation Pool, spent through governance).

**Decentralised self-governance is not ruleless chaos but the state in which the rules are written into code and no one can stand above them.** Every core rule (supply, distribution, slashing, thresholds, escrow, arbitration) is already fixed in code, and the "order" of self-governance comes precisely from code that anyone can audit and no one can secretly alter. This is the endgame form of the political philosophy of Chapter Six: when no one can stand above the rules, ownership returns truly and utterly to all under Heaven.

---

# Volume Four. The Distance: Imagining the Endgame of Economy and Politics

*This volume is about the future. It is not a promised endpoint but a road marked by real progress, and an endgame in which "every contribution deserves its reward".*

---

## Chapter Nineteen. The Distant Horizon of Commercial Value: Infrastructure for the Age of Decentralised AI

If the preceding chapters spoke of "why" and "how", this chapter speaks of "how much it is worth" — not the token price, but a structural opportunity that has been underestimated.

Humanity is moving into an age of acute hunger for AI compute. Demand for training and inference grows exponentially, while supply is locked by a handful of cloud vendors inside expensive data centres. It is a bridge with steep tolls, growing steeper. What MobileChain wants to do is to lay, alongside that bridge, a scattered, low-cost side road of compute built from billions of phones — it may not carry every workload, but it can carry that part of inference demand which "can be split into fragmented tasks and is not extremely latency-sensitive", at a price far below that of centralised cloud.

The commercial logic is clear: the demand side obtains inference compute at lower cost, and through on-chain escrow and arbitration gets settlement more transparent than a centralised API; the supply side (phone nodes) monetises idle compute, turning from "device holders" into "compute providers" for the first time; and as both sides grow, fees flow back to the security pool and real revenue takes the baton from the pre-minted vault — a self-consistent economy takes shape.

This is MobileChain's place in the "decentralised AI infrastructure" narrative: not to replace the cloud but to be its complement and its rival, scattering compute out of a few data centres and back into human pockets. Once this holds, "every phone is part of the infrastructure" turns from a slogan into a commercial reality that can be priced, verified and joined.

Let us spread this commercial ledger out a little further. The global cloud inference market is expanding at a double-digit rate each year, and the gross margins of the leading cloud vendors stay high year after year — and a large part of that excess is the premium of "centralisation": data must travel back to a central data centre, be computed on expensive GPUs, and then be sent back to the edge, and the bandwidth, latency and data-centre costs in between are all ultimately paid for by the demand side. A phone, however, is already where the data is produced, already in the user's hand, and when it completes a lightweight inference it saves precisely that whole "send back — compute — send back" chain. This means that for inference of equal quality, the theoretical cost on the edge can be far below that of central cloud — provided only that these scattered phones are organised into a trustworthy, settleable network. MobileChain's EdgeAI does exactly this work of "organisation".

The demand side has real pull as well. A great many AI tasks are not extremely latency-sensitive and can be broken into fragmented sub-tasks: image labelling, speech transcription, lightweight classification, data cleaning, sample generation for model fine-tuning… These are exactly the long-tail demands that phone compute is good at and that cloud vendors, finding them too "fragmentary", are unwilling to price carefully. When MobileChain connects this long-tail demand with the global supply of idle phones, it is not entering a red ocean monopolised by giants but a blue ocean that the giants cannot be bothered to stoop and pick up, yet which genuinely exists.

And so MobileChain's commercial story does not depend on a token-price narrative but on a plain chain of logic: real demand exists → edge supply exists → on-chain escrow and arbitration press the trust cost between the two sides to a minimum → once scale arrives, fees flow back to the security pool and real revenue takes the baton from the pre-minted vault → a self-consistent, sustainable economy whose ownership belongs to its participants takes shape. This, and only this, is the economic proof that "all under Heaven returning to all its people" can stand on its own feet.

---

## Chapter Twenty. Every "Point" in the Data Economy: The Return of Rights and Returns

This is the central proposition of the entire paper, and it deserves a chapter of its own, set down with deliberation.

MobileChain accepts a plain and firm premise: **in a network woven from billions of devices, every "point" that contributes presence, compute and data to the ecosystem is one of the true owners of that network, and deserves rights and returns commensurate with its contribution.**

This is not rhetoric; it is a hard constraint at the level of MobileChain's code:

- **Rights**: every phone that passes attestation and is genuinely online is an equal node of the network; its eligibility for DePIN device incentives depends not on anyone's approval but on its own real contribution. The attestation and sybil binding of `phonenode` protect exactly this: "one real device = one eligibility that cannot be claimed by another".
- **Returns**: for every verifiable task completed and every edge inference provided, the reward is paid out from the 550 million vault task by task according to real contribution, written into the public ledger, and belongs to the device owner. The design in which `depin` has no minting authority and only distributes guarantees that these returns come from real value, not from diluting others out of thin air.
- **Not intercepted**: the distribution rules are written into code, verified at genesis, and auditable by anyone; any use of Foundation funds goes through governance; the total supply cannot be inflated by governance. This means no centre can quietly move the participants' share into its own pocket.

Put these three layers together and you have the most concrete technical translation of that line from Chapter Six, "All under Heaven belongs to all under Heaven": **every measure of value in the network flows back along the path of real contribution to the very "point" that created it.** The data you produce is priced; the compute you provide is repaid; the presence you maintain is acknowledged. You are no longer a figure priced for free on a platform's ledger, but the owner of what you produce.

This is precisely the fundamental divide between MobileChain and the "digital feudalism" condemned in Volume One: there, users are the source of the means of production but not the destination of the returns; here, every "point" is both source and destination. A return of ownership, long overdue, is thus written into code.

It is worth taking this chapter one layer deeper, because it bears on MobileChain's highest value proposition. In the long history of humanity's struggle for workers' rights, every advance has meant drawing "the overlooked contributor" back into distribution: the Industrial Revolution freed workers from dependence on the guild master into free labourers, yet for a long time refused to recognise their right to organise and to share in profits; the labour movement of the 20th century won industrial workers the minimum wage, social insurance and collective bargaining. But when history entered the digital age, a more hidden kind of "labour" appeared — every output of yours on a platform is labour, yet it is not recognised as labour; the value you produce is measured, monetised and taken public, and only you have no share in it.

MobileChain does not claim to be a "digital labour movement", but what it does is structurally of the same origin: **to reconnect overlooked data labour to distribution.** The only difference is that it relies not on strikes and legislation but on code — writing "contribution is priced, pricing is repayment, repayment is booked" into a chain that no one can secretly alter. This is a quieter and far harder-to-reverse way of confirming rights.

Therefore, "every point that creates the data economy should receive its rights and returns" is not a marketing slogan but the design origin of every one of MobileChain's mechanisms. It shows up in:

- **In distribution** — 55% of the total supply is reserved for device incentives, the largest share aimed at the largest group of contributors;
- **In mechanism** — Resonance Distribution keeps incentives in step with real contribution, ruling out both "reward without work" and "work without reward";
- **Against tampering** — the total supply cannot be inflated by governance, any use of Foundation funds goes through governance, and parameters are auditable, ensuring that no centre can quietly move the participants' share away;
- **In ownership** — rewards are written into the public ledger and belong to the device owner, not to a platform's internal points.

When these four hold at once, the person whose phone screen lights up late at night becomes, for the first time and in both technical and institutional senses, the master of what they produce. And when billions of such "points" come together, they become the modern echo of that ancient proposition from Chapter Six — **all under Heaven will at last return to the hands of all its people.** This time, not by declaration, but by code.

---

## Chapter Twenty-One. Stages and Roadmap: Three Phases, Not a Pie in the Sky

MobileChain does not believe in "getting there in one step", only in "advancing in stages, verifiably".

**Phase One: Launch (roughly years 1–2)** — roll out the network, hold up security. The Device Incentive vault pays out against real tasks, and the first goal is to spread the phone node network across the globe; validator incentives come almost 100% as drip release from the Staking / Network Security Pool; governance has the team multisig carrying more of the coordination duties. An honest note: decentralisation in the launch phase is not yet sufficient, and MobileChain does not conceal this — decentralisation is a goal that takes time, not a state achieved the moment the chain goes live.

**Phase Two: Network Effect (roughly year 3)** — real revenue takes over, governance is handed over. Real EdgeAI tasks generate payer payments, and device node income shifts from "purely pre-minted" toward "pre-minted + real revenue"; fees flow back to replenish the security pool; governance is progressively and verifiably taken over from the team multisig.

**Phase Three: Maturity (roughly years 4–5 and beyond)** — the real economy drives on two wheels, the community governs itself. The pre-minted vault winds down; income comes mainly from real economic activity; fees become a major pillar of security; governance is essentially led by the community, and the team returns to being "ordinary participants + builders".

"Which year" is an indication of tempo, not a promise. MobileChain judges phase transitions by "milestone events" (such as fees first exceeding the security pool's drip release, or governance first autonomously approving Foundation spending). **Speak with real progress, not with a timetable that draws a pie.**

As for the real progress along this road, Chapter Twenty-Nine sets out the full three-tier list (done / in progress / planned), each item matching auditable code and on-chain state.

---

## Chapter Twenty-Two. The Vision: One Phone, One World Returned

MobileChain's vision can be compressed into a single sentence:

> **Let every phone in the world, at the lowest possible threshold, become a real part of an auditable public chain — and be rewarded for its own real contribution.**

The goal is plain — it talks of no disruption, no replacement; and it is vast — "every phone" means billions of people have, for the first time, the chance to truly take part in a public chain, and share in its value, without mining rigs, servers or great capital.

If MobileChain succeeds, we believe we will see: millions of phones forming a real decentralised device network; countless ordinary people earning transparent, verifiable returns from their real contribution through one phone's presence and compute; an economy that is zero-inflation, credibly scarce and yet durably secure, running on and on; idle compute becoming the infrastructure of a decentralised AI age; and control passing progressively and verifiably into the community's hands, as a genuinely self-governing network grows to maturity.

We must be equally clear about **what is not promised**: no promise of price, of rate of return, of an exact timetable, or that planned features will necessarily be delivered. We promise only one thing — **to advance honestly, to write every step into auditable code and documents, so that you can always verify for yourself how far MobileChain has come.**

A project worth lasting does not live on how loudly it trumpets the future, but on how solidly it builds the present. When that phone, on the bedside table late at night, quietly produces a block, completes an AI inference, and receives a modest payment of MC that comes from real contribution — the returning has already happened. And when tens of thousands upon thousands of such returns are stacked together, they become the modern echo of that ancient proposition from Chapter Six — **all under Heaven will at last return to the hands of all its people.**

If we pull the camera back to that "vessel of value" from the opening of this paper: from substance to metal, from paper to credit, from the centralised database to the public ledger, it has taken humanity five thousand years to make the recording and distribution of value, for the first time, capable of escaping the grip of any single centre. What MobileChain wants to do is press this final step into everyone's pocket — so that the hand holding the phone is at once the creator of value, the recorder of value, and the owner of value.

This may be the page that this history of the ledger has waited five thousand years for. It will not necessarily be finished by MobileChain — but MobileChain is willing to be the one that writes this page in earnest.

---

# Volume Five. The Engineering Fabric

*The first two volumes set out, at the macro and mechanism levels, MobileChain's "why" and "how the ownership returns". This volume expands the engineering detail that was condensed in Volume Two, for readers and auditors who wish to check it point by point. Every figure corresponds to a constant in the source code; wherever anything conflicts with the code, the code prevails.*

---

## Chapter Twenty-Three. Verifier Nodes: The Honest Scorer

Verification of DePIN task results is carried out by the verifier node role. It is the last gate on MobileChain's promise that "only real contribution is rewarded".

The power to score must have a price. If a role that can judge whether another's labour is up to standard has no collateral of its own that can be slashed, then cheating is free for it. So verifier eligibility is built on real collateral:

- **Collateral requirement**: self-bonded stake ≥ 30,000 MC, in the bonded state (`VerifierMinStake`) — the same collateral and the same slashing channel as a consensus validator, so a misjudgement or cheating directly cuts into principal;
- **Registration requirement**: node registration completed in `phonenode`, device attestation in the valid state, and heartbeat falling inside the offline grace window;
- **Slashing reach**: principal deduction (40% burned / 60% returned to the Staking / Network Security Pool) is triggered only for verifiers in the bonded state; if a registered node that holds no verifier self-bonded stake misbehaves, the protocol merely revokes its device attestation and records a slashing entry, without deducting any principal — what rides on it is only the eligibility to keep receiving tasks, not on-chain assets.
- **Duties**: receive randomly assigned task submissions, re-run off-chain the verification script provided by the AI project, and submit the resulting hash on-chain, for comparison against the submitter's result hash in the on-chain judgement;
- **Selection**: drawn from the candidate set meeting the above conditions, weighted by reputation (weight = reputation score 0–100), with the randomness derived from block data and determined network-wide; a node whose reputation is deducted to 0 has its weight set to zero and drops out of the sampling queue automatically;
- **Returns**: each verification earns 15% of that task's bounty (escrowed by the payer when the task is posted and paid out from the escrow amount at settlement, without touching the device vault).

**Duties independent of block production**: verifier nodes judge whether task results are right or wrong, while consensus validators decide the order of blocks. Both are carried by the same pool of stakers, yet they are two independent duties that do not substitute for each other — collateral is the shared foundation of responsibility, judgement is the separated power.

**Verification script spec**: Python 3.10+, runtime ≤ 30s, no network requests permitted, standard JSON output; the script hash is stored on-chain against tampering (checked when results are submitted); the judgement logic is settled by the on-chain module according to hard-coded rules, and verifier nodes cannot tamper with it.

**Division-of-labour chain**: AI project posts a task → phone nodes claim it, run inference, and submit → N verifier nodes are randomly assigned (3 per round by default) → each re-runs the verification script off-chain and submits a result hash → the chain compares the verifier hashes with the submitter's hash (match = pass / mismatch = cheating, entering dispute) → on pass, settlement proceeds (executing node 85% / verifier nodes reserve 15%, with no burn deduction); on a ruling of cheating, payment is refused and the executing node's reputation is deducted.

---

## Chapter Twenty-Four. The Native Exchange: Where Liquidity Is Born and Deflation Is Driven

MobileChain ships with a native AMM module, `x/dex`, built on the constant-product (x×y=k) market-making model, supporting pool creation, token swaps, and the addition and removal of liquidity.

| Parameter | Value |
|---|---|
| Initial liquidity pool | 5 million MC + 100,000 USDT (the MC/USDT primary pair) |
| Initial price | 0.02 USDT/MC |
| Trading fee | 0.30% (half of it goes to the burn address, equal to 0.15% of the trade; the other half goes to liquidity providers) |
| Liquidity incentive (first 6 months) | 5,000 MC/day (drawn from the Device Incentive Pool, governance-adjustable) |
| Minimum LP lock-up period | 100,800 blocks (about 4.7 days at ~4-second blocks) |

The MC side of the initial liquidity pool is drawn as 5 million MC from the Foundation's T0 unlock, while the USDT side is provided by a market-maker partner. An initial price of 0.02 USDT/MC corresponds to an initial circulating market capitalisation of roughly $2 million. The DEX is not only where MC is discovered in price; it is also a deflationary engine — half of the 0.30% fee on every trade (0.15% of the trade) is burned forever, while the other half stays in the pool as the reward of liquidity providers, with the treasury taking no cut. The arrival of liquidity means that, for the first time, the MC earned by this phone can be exchanged for its fiat equivalent on a real market; and deflation quietly makes every holder heavier with each trade.

---

## Chapter Twenty-Five. Referral Incentives: Symbiosis Without Skimming

MobileChain's referral incentives use a **ten-generation tiering** model, already delivered as a native module (`x/referral`): the reward source is never deducted from the referee's earnings, but paid out additionally from a budget set aside in the Device Incentive Pool.

**Ten-generation tiering**:

| Parameter | Value | Notes |
|---|---|---|
| First-generation reward (direct invitation) | 10% | 10% of the referee's DePIN earnings, paid by the system to the referrer on top |
| Second-generation reward (one level of indirect referral) | 5% | the person invited by the referrer's referrer |
| Third-generation reward (two levels of indirect referral) | 3% | one level deeper still |
| Fourth-generation reward | 2% | the fourth level |
| Fifth-generation reward | 1% | the fifth level |
| Sixth- to tenth-generation reward | 0.5% each | the sixth through tenth levels, 0.5% per level |
| **Total ten-generation weight** | **23.5%** | the ceiling on what the network pays to acquire one participant |
| Mining node earnings depth | **a full 10 generations** | nodes registered + with valid attestation + heartbeat within the offline grace window |
| Non-mining node earnings depth | **the first 5 generations** | accounts that have not become mining nodes enjoy only generations 1-5 |
| Per-person daily reward ceiling | 500 MC | to prevent any single referrer from profiting excessively |
| Network-wide daily total ceiling | 20,600 MC | a global circuit breaker |
| Referral budget share (within the Device Incentive 55%) | 15% | governance-adjustable |
| Relationship binding | permanent and irrevocable | to prevent the buying and selling of relationships |

**The ten-generation weight table** (generations 1-10 at 10%/5%/3%/2%/1%/0.5%×5, in basis points `1000/500/300/200/100/50×5`, totalling 23.5%) is the ceiling on the network's cost of acquisition: when a referee generates earnings, the system traces up the referral chain as far as ten generations, paying the corresponding share as an **additional** reward from a separate budget pool to each upstream referrer; the referee keeps 100% of their own earnings, and no referral commission is ever deducted from the referee's earnings or eats into their share.

**Node identity determines earnings depth**: mining nodes (registered on phonenode, with valid device attestation, and a heartbeat within the offline grace window) enjoy the full 10 generations of earnings; accounts that have not become mining nodes enjoy only the first 5. This design makes "actually connecting a device to the network and keeping it online" the precondition for unlocking the deeper generations — deep-generation rewards flow only to real builders.

The 15% referral budget sits within the 55% Device Incentive share, a total budget of roughly 82.5 million MC; the daily ceiling of 20,600 MC would sustain it for about eleven years even at full speed, and actual consumption tracks real activity. The referral module has no minting authority, and rewards are triggered in step with DePIN / EdgeAI settlement: trace the relationship chain (up to 10 generations) → compute by generational weight → verify node identity and the circuit breaker → allocate from the separate budget pool → the referrer receives 100% of the full amount, with no burn deduction on-chain. The fundamental difference from the traditional model: the reward source is a pre-minted, separately set-aside independent budget, the referee's own earnings are untouched to the last fraction, the system budget has a ceiling, and the community can adjust rates and depth by governance at any time.

---

## Chapter Twenty-Six. Off-Chain Services and Oracles: A Constrained Messenger

A blockchain is good at recording and verifying, yet it cannot directly perceive the off-chain world. Many of MobileChain's core judgements depend on off-chain facts: whether a device is truly online, whether an AI inference was truly completed, whether an attestation can be trusted. These facts are produced off-chain and must be brought on-chain in a secure and auditable way.

Oracle service hardening: Bearer token authentication (only legitimate data sources holding a valid token may submit), rate limiting (to prevent high-frequency malicious submission), and optional TLS (transport-layer encryption). The role boundary is clear: **it only submits verifiable off-chain facts on-chain in a controlled way; the final issuance, slashing, and settlement are still executed by on-chain modules under hard-coded rules.** An oracle is not a centre of power, but a constrained messenger.

For high-frequency, small-value scenarios such as device incentives and AI settlement, putting every transaction on the main chain is both expensive and congesting. MobileChain is **planning** to explore off-chain channels and batch settlement: high-frequency interaction off-chain, with net results periodically settled on-chain in batches, balancing efficiency and security, while final settlement is still guaranteed fair by on-chain escrow and rules.

---

## Chapter Twenty-Seven. Product Philosophy: Keep the Complexity in the Code, Keep the Simplicity for the User

However ingenious a chain may be, if ordinary people cannot use it, it cannot deliver on "returning it to the majority". MobileChain's product principle: **on-chain logic may be very complex, but the operations a user sees must be as simple as possible.**

Components already in place: the command-line tool `mcchaind`, a Web dashboard (wallet + block explorer), a read-only query gateway (gRPC-gateway REST), a transaction helper (the front end generates a copyable `mcchaind tx` command), and wallet interoperability (standard Keplr registration, prefix `mc`, SLIP-44 = 118).

**MC Miner APP interaction design** (a phone full-node APP for ordinary users):
- **Home — a live-globe node heat map**: a 3D rotating globe covered in points of light, flashing once every 4 seconds (the birth of a new block), showing the network's node count, today's total DePIN earnings, the current block height, and statistics by region. A huge "Start Mining" button draws you in — the scene itself is the marketing material.
- **A 5-minute onboarding (zero jargon)**: create a wallet → claim 100 MC from the faucet → complete your first DePIN task (1 minute of AI inference) → "Congratulations! You rank No. XX among nodes worldwide", with the words "blockchain / BFT / private key / consensus" never appearing along the way.
- **Smart notifications**: instant reach for DePIN completion, a referrer earning money, a rise in ranking, a new governance proposal, MC price movement, a node going offline, milestones, and the like.
- **A social honour system**: achievement badges (Genesis Miner / Compute Pioneer / Evangelist / Top-100 Node / Loyal Guardian) and team leaderboards.
- **USD-equivalent display**: the USD equivalent shown in real time beneath the balance, lowering the cognitive threshold.
- **In-app FAQ**: setting "the truth" against "common misconceptions".

The mobile SDK integration guide walks developers through device attestation, keeping a node alive, submitting task contributions, and securely managing keys — the last mile that carries "phone full node" from on-chain capability into users' hands.

---

## Chapter Twenty-Eight. The Initial Ecosystem Development Fund and Bug Bounty

What a chain lacks most in its early days is not a vision, but the resources to push that vision into reality. The fund comes mainly from the **Foundation Pool (13% = 130 million MC)**, used for developer incentives and ecosystem grants, security audits, mobile and infrastructure building, and community education and operations.

MobileChain plans to launch a public bug bounty (through platforms such as Immunefi) on the day the mainnet goes live: critical (theft of funds / unlimited minting) up to 50,000 USDC, high (halting or forking the chain) 25,000 USDC, medium (bypassing limits / forging data) 5,000 USDC, low 1,000 USDC. The bounty pool is drawn from the Foundation's T0 unlock.

An iron rule: **every use of the Foundation Pool should go through an on-chain governance proposal and a community vote.** A centralised vault free of governance constraints, however good its intentions, violates "open-source and auditable". Handing the right to deploy 13% to governance means the money ultimately belongs to the community, serves the community, and is overseen by the community — it is an "Initial Ecosystem Development Fund", not "the team's slush fund". The Early Development Pool (5%) rewards, with precision, the substantive contributors before and after mainnet, standing alongside the Foundation Pool to cover both ends of the ecosystem.

---

## Chapter Twenty-Nine. The Roadmap in Full

In keeping with the principle of "truth on-chain", the roadmap is marked honestly in three tiers:

- **Completed ✅**: the eight native modules (including liquid staking, the native AMM, referrals, and the gradual handover of governance), end-to-end verification of the Five Pools economy (55/15/12/13/5), the zero-inflation model, the phone node security system (attestation / sybil binding / offline grace / tiered slashing / cooldown), the full EdgeAI flow (publication / escrow / optimistic settlement / arbitration), the validator threshold (a 30,000 MC ante), oracle hardening (Bearer + rate limiting + TLS), the Web dashboard, complete documentation, IBC interoperability and the CosmWasm contract layer, and off-chain batch settlement.
- **In progress 🔄**: mainnet launch preparation (genesis / seed nodes, validator recruitment, final audit), configurable Web RPC and experience optimisation, and polishing the mobile SDK.
- **Planned 📋**: scaling the EdgeAI compute marketplace (all remaining protocol-layer capabilities are already delivered).

The roadmap is updated with real progress and measures advancement by "the achievement of milestone events" rather than "promised dates" — always take the on-chain state and the open-source code as your reference.

---

## Chapter Thirty. Value Capture: Beyond Scarcity, Usefulness Is Needed

MC is not a claim on the earnings of any entity, and it promises no returns. What it has is a total supply locked by code, and those places in the network's own operation where it must be consumed.

**Structural demand comes from four channels that cannot be bypassed.** Every transaction pays gas in MC; every validator stakes 30,000 MC in self-delegation, and every mobile node stakes collateral that can be slashed; every enterprise-grade AI task escrows its budget on-chain in MC before a phone takes the order; and one side of every liquidity position on the native exchange is MC. None of these four is an initiative; each is a cost of using this network.

**Structural reduction has only three paths, and all are hard-coded in constants.**

| Path | Ratio | Charging basis | Source constant |
|---|---|---|---|
| Gas fee | 7.00% of the fee | every transaction | `GasBurnRatioBps = 700` |
| Exchange | half of the 0.30% fee, i.e. 0.15% of the trade | every swap | `FeeBurnBps = 5000` |
| Slashing | 40% of the slashed amount | validator misbehaviour | `SlashBurnRatioBps = 4000` |

Burned MC goes to a deterministic, unspendable address. It does not stay in the treasury, and governance cannot take it back.

**What is deliberately not burned: the earnings of labour.** Device rewards, referral rewards, and EdgeAI settlement arrive in full at the participants; no share is taken from the fruits of work. Deflation is supplied by protocol usage and misbehaviour, not by anyone's labour.

**What the protocol does not do.** There is no treasury buyback, no market-making promise, no dividend right, and no staking yield propped up by issuance. The drip-feed yield comes from a pre-designated 150 million MC Staking / Network Security Pool; when the pool is exhausted, the yield it supports ends with it. A renewal mechanism can lower the rate to extend the years, but it cannot conjure supply out of nothing.

The honest statement is this: how much value MC captures depends on how far this network is used. The supply side has been settled by code; the demand side is not something a document can settle.

---

## Chapter Thirty-One. Diffusion and Acquisition Budget: Writing Growth as Auditable Arithmetic

The growth section states only the budget and the gates, because those two things can be checked from the outside.

**Where the acquisition budget comes from, and its ceiling.** The referral incentive is carried by an 82.5 million MC budget cut out separately from within the Device Incentive Pool — an independent budget, paid to the referrer (the upline), and never deducted from the earnings of the referee (the downline). The ten-generation weights are 10%, 5%, 3%, 2%, 1%, 0.5%×5 (`DefaultLevel1..10RewardRateBps`, 23.5% in total), so **23.5% is the ceiling on what the network pays to acquire one participant**, and it is disbursed from the 82.5 million pool, independent of that participant's own earnings; non-mining nodes enjoy only the first 5 generations (21% in total).

Two circuit breakers constrain the spend rate: 500 MC per referrer per day, and 20,600 MC across the network per day (`DefaultDailyNetworkCap`). In the extreme case of full utilisation, the network-wide cap alone spreads the 82.5 million budget over 82,500,000 ÷ 20,600 ≈ 4,005 days, roughly eleven years. No single growth pulse can drain it — that conclusion is a division between constants, not a promise.

**The scale the device budget can support.** 467.5 million MC covers the twelve-year drip-release floor, an average of about 106,700 MC per day. Against this, the node capital allowance is paid at 30 MC per day for each registered node with valid attestation that has not been jailed (`DefaultNodeCapitalAllowancePerDay`). The two numbers are joined by a single governance dial: the allowance is a per-node constant, total spending grows linearly with the number of nodes, and so the per-node amount ought to be lowered as the network expands. It is a governance parameter, not a fixed right; its consequences are stated plainly in Section II of Chapter Thirty-Two.

**The payment structure on the demand side.** A single enterprise task is escrowed at a maximum of 1000 MC (`MaxTaskReward`); on settlement, 85% goes to the device that submitted the result and 15% goes into the verification reserve, with a further 1.50% settlement fee split 40% to node operators and 60% to the protocol treasury. The treasury therefore receives 0.90% of enterprise settlement volume. This is linear arithmetic, and the reader can substitute any magnitude they accept: at a daily settlement volume of 1 million MC, the treasury takes in 9,000 MC per day. This is arithmetic, not a forecast.

**Gates in place of dates.**

- *First gate: genesis integrity.* The validator set is large enough that the failure of any single member is not fatal, and the 30,000 MC self-bond is enforced by an ante decorator on every create/edit-validator transaction. Measure it by the on-chain validator set, not by announcements.
- *Second gate: real demand.* Enterprise settlement fees become recurring revenue for the treasury, so that the treasury is fed by "use" rather than by genesis unlocks. Measure it by the composition of the treasury balance at any given height.
- *Third gate: the handover of power.* The operational keys held by the team diminish, parameters are transferred item by item to on-chain voting, and the treasury withdrawal timelock is enabled. Measure it by "what has already been handed over", not by "what has been announced as going to be handed over".

---

## Chapter Thirty-Two. Risk Disclosure and Limitations: A Document That Lists Only Strengths Is Not an Engineering Document

Every one of the following is a currently known and genuinely existing limitation.

### I. Risks of the Launch State

**Genesis key material.** The team share is held by a 3-of-5 multisig, with member public keys injected at build time. The early development address and the two foundation addresses use the same mechanism, resolved through explicit public key overrides. If an override is missing, the build falls back to a deterministic address derived from a fixed seed — a placeholder address that anyone reading the source can reproduce, **and which must therefore never hold assets**. Replacing every placeholder address with a real public key is a blocking precondition for the genesis ceremony. Until the replacement results can be verified in the genesis file, no share figure in the appendix should be understood as a statement about the state of custody.

**External audit.** The protocol is covered by the test suite in the repository (if an economic constant drifts, the tests fail) and has been through internal review. **A third-party security audit report is not yet complete.** Until the audit report is public, treat this code as not independently audited by a third party.

**Validator concentration.** At genesis the validator set is necessarily small. The smaller the set, the easier it is to coordinate, and the easier to capture. The 30,000 MC self-bond threshold raises the cost of "spinning up a validator on a whim", but no parameter can substitute for a genuinely decentralised set, and decentralisation takes time — a stretch of time the protocol cannot compress.

### II. Economic Risks

**The allowance grows with the network; the pool does not.** The node capital allowance is a fixed payment per node per day, funded from the fixed 467.5 million device pool. Total spending grows linearly with the number of nodes, while the pool is a fixed quantity. If the node count grows faster than governance lowers the per-node amount, the pool will run dry before the twelve-year floor. The mitigation is a governance parameter that must actually be used — it does not take effect automatically, and the protocol will not throttle itself.

**The endgame of the drip release.** The staking drip release targets 5% annualised on bound MC, funded from a finite pool of 150 million MC, over a twelve-year floor. A renewal band can lower the target to 1.00%–2.00% in order to extend the term. Both ends are reductions in yield, and neither adds to supply. Any modelling of staking returns should be built as "declining year over year".

**Zero inflation means the security budget cannot expand elastically.** Zero inflation is a deliberate constraint, and its cost is that the security budget cannot be enlarged by a stroke of a resolution. The long-run validator economy depends on fee volume and the balance of the security pool; if both decline at once, the willingness to validate declines with them.

**The inherent cost of the referral mechanism.** Any mechanism that rewards referrals will attract participants whose only activity is referring. The per-person daily cap, the network-wide daily cap, the ten-generation depth ceiling (the first 5 generations for non-nodes), the maximum of 100 referees per account, the minimum payout threshold — these circuit breakers bound the scale of the damage, not the disappearance of the behaviour itself.

### III. Technical Risks

**The off-chain surface of trust.** Device attestation and online status are facts the chain itself cannot observe. The oracle reports under constraints — Bearer token authentication, rate limiting, fail-closed verification against the hardware root of trust, mandatory public keys in production. Constraints are not elimination: once oracle credentials leak, that is a real attack surface. The honest description of the boundary is this: the oracle is responsible for reporting, and the on-chain module is responsible for adjudicating.

**The exchange settlement operator.** High-frequency micro-payments go through off-chain batching and a single on-chain settlement. Both the submission and the clearing of a batch are restricted to authorised addresses (by default the governance module account), and governance holds a circuit-breaker switch across the entire path. At submission, each batch commits a SHA-256 digest to the set of entries; at clearing, the digest is recomputed and payment is refused on any mismatch, so that "the entries that were cleared" are provably equal to "the entries that were submitted". The residual risk lies in the availability of the authorised submitter: if it stops submitting, micro-settlement halts until governance appoints a new submitter.

**Liquid staking.** The ulmc voucher is a claim on a pooled delegation, not a deposit, and is priced by redemption rate rather than face value. Compounded staking yield lifts that rate; slashing of a validator depresses it. The write-down is completed at the instant `x/staking` reports a slash, earlier than any redemption is processed — deliberately so: the loss is shared across all holders rather than landing on whoever redeems last. In the extreme case of a full write-down, only unbacked shares remain in the pool, and in that state new deposits are refused until governance disposes of the pool.

**Smart contract execution.** CosmWasm runs only on CGO-enabled builds (the WebAssembly runtime is linked into that build). Contracts are third-party code, and the protocol makes no statement whatsoever about any contract deployed on it.

**Cross-chain transfer.** IBC connects MobileChain to counterparty chains, and the security of those counterparty chains is not guaranteed by MobileChain. Assets that cross in carry the risk of their origin.

### IV. Governance Risks

Treasury spending requires the governance multisig, and that is already enforced and in effect today. **The withdrawal timelock is still on the roadmap and not yet enabled**: in v1 there is no mandatory delay between approval and the movement of funds. Until the timelock goes live, the only control is the multisig itself.

Governance capture is an inherent risk of any token-weighted voting system. The supply cap and the permanent tombstone for malfeasance sit beyond the reach of governance; most other parameters do not.

### V. External Risks

Jurisdictions differ in how they characterise network tokens, and the characterisations are still changing. Exchange availability, custody, taxation and the legality of participation itself all lie outside the control of the protocol. Market liquidity may be thin, and the price of MC may go to zero.

---

## Chapter Thirty-Three. Legal and Regulatory Posture: Verifiability in Place of Guarantees

**What MC is.** MC is the native unit of account of a public, permissionless network. It pays gas, serves as the bond for validators and nodes, escrows EdgeAI tasks, and forms one side of liquidity on the native exchange. Its utility ends with these functions.

**What MC is not.** MC does not represent equity, does not represent ownership of any entity, and carries no dividend, interest, claim on earnings or right to repayment of principal. Holding MC creates no relationship with any issuing entity. The various rewards described in this document are protocol-level distributions executed by on-chain rules against pre-delineated pools; they are not returns on investment, and no one stands behind them.

**This document is not an offer.** This document is a descriptive document of technology and economics, and in no jurisdiction does it constitute an offer to sell, an invitation to make an offer to buy, or a recommendation regarding any asset. This document does not constitute investment, legal, accounting or tax advice.

**Participants are responsible for their own participation.** The protocol is software. It does not screen participants, and it cannot determine whether participation is lawful in the place where a given participant is located. Exchanges, custodians, front ends and other intermediaries bear their own obligations under their own regulatory regimes; running a node, deploying a contract or trading MC may be restricted or prohibited in certain jurisdictions. Compliance and tax obligations rest with each participant.

**A commitment to transparency in place of a commitment to guarantees.** The team and foundation shares are released on an on-chain schedule, enforced by code rather than by promise; the balance of every pool is queryable at any height; and every constant in the appendix is annotated with the file that defines it. Wherever this document and the code disagree, the code prevails — and that rule is itself the compliance posture: everything stated about MobileChain should be verified against on-chain state, not accepted from a document.

**Forward-looking statements.** Statements about the roadmap, diffusion or future capabilities express intent, not promise. They depend on engineering outcomes, governance decisions and external conditions, and may not come to pass.

---

# Appendix. The Sanctum of Parameters (Auditable Line by Line Against the Source Code)

This appendix gathers every key parameter cited in the main text and marks its source location. Auditors can use it to check, item by item, that "the whitepaper numbers = the code numbers". **Where anything differs from the code, the code prevails.**

## A.1 Chain-Level Base Parameters

| Parameter | Value | Source Location |
|---|---|---|
| Consensus engine | CometBFT v0.37.6 | `go.mod` |
| Application framework | Cosmos SDK v0.47.14 | `go.mod` |
| Language | Go 1.21 | `go.mod` |
| Mainnet chain ID | `mcchain-mainnet-1` | Genesis configuration |
| Account address prefix | `mc` | `app/app.go` |
| Smallest unit of the base coin | `umc` | `x/tokenomics/types/keys.go` |
| Precision | 6 (1 MC = 1,000,000 umc) | Chain configuration |
| Additional issuance inflation | 0 (`x/mint` forced to zero) | Genesis/mint configuration |

## A.2 Tokenomics

| Parameter | Value (umc) | Value (MC) | Source Constant |
|---|---|---|---|
| Total supply cap | 1,000,000,000,000,000 (1e15) | 1 billion | `TotalSupplyCap` |
| Device incentives 55% | 550,000,000,000,000 | 550 million | `DeviceIncentivePercentBps=5500` |
| — of which referral ecosystem budget | 82,500,000,000,000 | 82.5 million | `ReferralEcosystemBudget` |
| — of which device reward vault | 467,500,000,000,000 | 467.5 million | `DepinInitialPoolSlice - ReferralEcosystemBudget` |
| Staking security 15% | 150,000,000,000,000 | 150 million | `StakingSecurityPercentBps=1500` |
| Team 12% | 120,000,000,000,000 | 120 million | `TeamPercentBps=1200` |
| Foundation 13% | 130,000,000,000,000 | 130 million | `FoundationPercentBps=1300` |
| — genesis instant unlock | 50,000,000,000,000 | 50 million | `FoundationT0Unlock` |
| Early development 5% | 50,000,000,000,000 | 50 million | `EarlyDevPercentBps=500` |
| Device pool slice (before split) | 550,000,000,000,000 | 550 million | `DepinInitialPoolSlice` |
| DEX initial liquidity | 5,000,000,000,000 | 5 million | `DexInitialLiquidityMC` |
| Protocol treasury genesis balance | 0 | 0 | `ProtocolTreasuryPoolName` |
| Team multisig threshold | 3-of-5 | — | `TeamMultisigThreshold=3` |
| Team release | 1-year cliff + 3-year linear | — | `x/tokenomics/keeper/genesis.go` |

## A.3 Device Incentives (depin)

| Parameter | Value | Source Constant |
|---|---|---|
| Initial vault | 467,500,000,000,000 umc (467.5 million MC) | `DefaultInitialPool` |
| Reward denom | `umc` | `DefaultRewardDenom` |
| Minting authority | None (distribution only, no minting) | maccPerms (no Minter) |
| Contribution quality threshold | 30 | `ContributionThreshold` |
| Consistency with tokenomics | `DepinInitialPoolSlice - ReferralEcosystemBudget == DefaultInitialPool` (genesis assertion) | `x/tokenomics/keeper/genesis.go` |

## A.4 Phone Node Security (phonenode)

| Parameter | Value | Meaning | Source |
|---|---|---|---|
| `AttestationRequired` | true | Attestation required to participate | `x/phonenode/types/params.go` |
| `AttestationValidity` | 2,592,000 seconds (30 days) | Attestation validity | same as above |
| `SybilDeviceBinding` | true | Sybil device binding | same as above |
| `OfflineGraceBlocks` | 100 blocks | Offline grace | same as above |
| `OfflineSlashBps` | 500 (5%) | Offline slashing | same as above |
| `ContribSlashBps` | 1000 (10%) | Cheating-contribution slashing | same as above |
| `AttestSlashBps` | 2000 (20%) | Forged-attestation slashing | same as above |
| `SlashCooldownBlocks` | 43200 blocks (about 12 hours) | Cooldown before re-attestation after slashing | `DefaultSlashCooldownBlocks` |

## A.5 Edge AI (edgeai)

| Parameter | Value | Meaning | Source |
|---|---|---|---|
| `MaxTaskReward` | 1,000,000,000 umc (1000 MC) | Per-task bounty cap | `x/edgeai/types/params.go` |
| `DisputePeriodBlocks` | 100 blocks | Dispute window | same as above |
| `AntiCheatThresholdBps` | 5000 (50%) | Anti-cheat threshold | same as above |
| `Arbitrator` | Team multisig address (set at deployment, can be brought under governance) | Dispute arbiter | same as above |
| Payment model | Payer-funded escrow + optimistic settlement | — | `x/edgeai` keeper |

## A.6 Consensus Security Threshold (ante)

| Parameter | Value | Meaning | Source |
|---|---|---|---|
| `MinSelfDelegationLowerBound` | 30,000,000,000 umc (30,000 MC) | Validator minimum self-delegation | `app/ante.go` |
| Enforcement scope | Chain-wide, on any validator-create/modify transaction | — | `MinSelfDelegationDecorator` |

## A.7 Module and Code Structure Inventory

| Module or Component | Path | Responsibility |
|---|---|---|
| mcchain | `x/mcchain` | System anchor module |
| tokenomics | `x/tokenomics` | Total supply cap, Five Pools allocation, team vesting |
| depin | `x/depin` | Device incentive vault and per-task distribution |
| phonenode | `x/phonenode` | Device attestation, sybil binding, tiered slashing |
| edgeai | `x/edgeai` | AI tasks, escrow payment, optimistic settlement, dispute arbitration |
| dex | `x/dex` | Constant-product AMM and fee burn |
| referral | `x/referral` | Referral ledger and dual circuit breakers |
| liquidstaking | `x/liquidstaking` | Delegated liquid staking and staking yield |
| cosmwasm | `x/wasm` | WebAssembly smart-contract runtime (enabled with CGO builds) |
| ibc | `x/ibc` (ibc-go) | Interchain accounts and IBC transfers |
| ante decorator | `app/ante.go` | Validator minimum self-delegation enforcement |
| Off-chain oracle | `internal/oraclesvc` | Controlled submission of off-chain facts on-chain |
| Web dashboard | `web/` | Wallet + block explorer + transaction assistant |

## A.8 Liquid Staking and Off-Chain Settlement

| Parameter | Value | Source |
|---|---|---|
| Receipt token | `ulmc` | `x/liquidstaking/types/keys.go` |
| Minimum liquid staking amount | 1,000,000 umc (1 MC) | `DefaultParams().MinStakeUmc` |
| Per-validator concentration cap | 2000 bps (20% of module delegations) | `MaxValidatorShareBps` |
| Slashing pass-through | Immediately writes down pool principal when `BeforeValidatorSlashed` fires | `x/liquidstaking/keeper/hooks.go` |
| New deposits after endorsement is wiped out | Rejected (`ErrPoolWipedOut`) | `x/liquidstaking/keeper/liquidstaking.go` |
| Source of yield | Staking yield reinvested, no new issuance | `x/liquidstaking/keeper/liquidstaking.go` |
| Settlement batch submitter | Governance module account by default | `x/dex/keeper/settlement_config.go` |
| Settlement circuit breaker | Set by governance | same as above |
| Batch integrity | Commits the SHA-256 of the entry set at submission, recomputed and verified at clearing | `x/dex/keeper/settlement.go` |

---

# Appendix B. How to Verify Every Claim in This Document

1. Clone the repository and build the client yourself. The build process is reproducible from `go.mod`.
2. Open `x/tokenomics/types/keys.go`. Every allocation ratio and rate constant in Appendix A is declared there.
3. Run the test suite. Genesis allocation, total supply cap enforcement, each fee split, and the slashing paths are all covered by tests; if any number drifts, the tests fail.
4. Query module account balances from a running node. The Five Pools and the treasury are all addressable, and their balances are public at any height.
5. Compare what you find against this document. If anything differs, the code prevails.

---

*This whitepaper is an explanation of the MobileChain code, not a promise about the code. Where anything differs from the code, the code prevails.*
