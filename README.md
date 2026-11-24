# Consignment note register for road transport offices

A free, offline, single-binary tool that issues numbered consignment notes for
domestic road carriage — the bilty, the lorry receipt — in their three
conventional copies, follows each consignment from booked to delivered, raises
a proof of delivery from the delivery record, and audits the number series for
gaps and reuse on every read.

## What it does

The bilty is the one document that must exist. It travels with the goods, it is
the evidence of the contract of carriage, and issuing it is what makes a carrier
a goods transport agency. Most small offices still write it in a triplicate book
with a carbon sheet, which cannot be searched, is easily lost, and whose holes do
not show.

- **Issues a numbered consignment note** with the consignor, the consignee, the
  goods as declared, the packages, the weight, the freight term, the declared
  value, and the vehicle and driver — on a number allocated from a series the
  tool controls.
- **Prints three distinct copies.** The consignor's, the consignee's and the
  carrier's are not photocopies of each other: each carries what that party needs
  and not what it does not. A prepaid consignee's copy shows no freight at all,
  because nothing is collected there and the rate the consignor negotiated is not
  the consignee's business — and the copy says on its face why the figure is
  absent.
- **Audits the number series, loudly.** A gap or a reused number puts a red
  banner at the top of the register, names the number, and says what to do about
  it. Nothing is ever deleted: a consignment that was called off is *cancelled*
  and keeps its number, and a spoiled leaf is recorded as *void* with a reason.
  Allocation never fills a gap, and renumbering only goes forwards.
- **Tracks the consignment** through booked, in transit, arrived and delivered,
  refusing the moves that make no sense, and raises a **proof of delivery** that
  states what it evidences and — just as plainly — what it does not.
- **Says who owes the freight**: paid, to pay, or billed to a third party, with
  the register totalling what the delivery clerk is due to collect at the door
  across every consignment still out.
- **States the tax position rather than guessing it.** A note whose position has
  not been settled prints that it has not been settled.
- **Keeps a register you can search** by number, party, route, goods, vehicle or
  way bill number, and export as CSV along with the serial audit.

**It does not generate an e-way bill.** The electronic way bill is a separate
document, raised and filed on the government portal. The tool has a field to
record the number you were given so that the note and the way bill can be found
together, and that field is the whole of its involvement.

## Who it's for

Goods transport agencies and small transport booking offices in India — whoever
sits at the counter and writes the note, and whoever has to answer a month later
for where number 0147 went.

## The rule it implements

A consignment note is not a form anybody publishes. What it is, under the goods
and services tax, is a term of art with consequences.

- A **goods transport agency** is defined as a person who provides service in
  relation to transport of goods by road *and issues a consignment note, by
  whatever name called* — clause (ze) of paragraph 2 of **Notification No.
  12/2017-Central Tax (Rate) dated 28 June 2017**. Issuing this document is the
  thing the definition turns on.
- Tax on a goods transport agency's service is **commonly payable by the
  recipient under reverse charge**, where the recipient falls within the
  categories specified by **Notification No. 13/2017-Central Tax (Rate) dated 28
  June 2017, as amended**. The agency may instead elect to pay the tax itself
  under forward charge, at a rate depending on that election and on whether it
  takes input tax credit, under **Notification No. 11/2017-Central Tax (Rate)
  dated 28 June 2017, as amended**.
- **Which of those applies is not something this tool decides.** It depends on
  the agency's own election and the status of the recipient of the service,
  neither of which the tool can see. You choose the position; the tool works out
  the figure that position implies, on the gross amount charged for the
  transportation of those goods, from a rate held as a named constant you can
  change on screen. Every figure carries its caveats with it.
- The **particulars** printed — a serial number, the consignor and consignee, the
  vehicle's registration number, the details of the goods, the place of origin
  and the place of delivery, and who is liable to pay the tax — follow the
  long-standing trade convention descending from the particulars prescribed by
  **rule 4B of the Service Tax Rules, 1994**. Goods and services tax law uses the
  term *consignment note* without re-prescribing that list in the same form, so
  treat these as the trade's usual contents rather than a statutory form.
- The **copy set** is the practice of the triplicate bilty book, and sits
  alongside the obligations of a common carrier under the **Carriage by Road Act,
  2007**. This tool prints the set; it does not reproduce the Act.

### The statutory figures this build carries

Each is a **named constant in one place**, copied into your register file and
**changeable on screen**, so a change to the law is a change to a number and not
a change to the program.

| Constant | This build | What it is |
| --- | --- | --- |
| `RateRecipientReverseChargeDefault` | 5.00% | Rate where the recipient accounts for the tax under reverse charge. Notification No. 13/2017-CT(R), read with 11/2017-CT(R), both as amended. |
| `RateGTAForwardNoITCDefault` | 5.00% | Rate where the agency pays under forward charge without input tax credit. Notification No. 11/2017-CT(R), as amended. |
| `RateGTAForwardWithITCDefault` | 12.00% | Rate where the agency pays under forward charge with input tax credit. Same reference. |
| `ExemptSingleCarriageGrossDefault` | ₹1,500 — **not applied** | Small-consignment figure from the entry against serial number 21 of Notification No. 12/2017-CT(R), **since amended**. Off by default. |
| `ExemptSingleConsigneeGrossDefault` | ₹750 — **never applied** | The per-consignee equivalent. Carried for reference only. |

**All of these are indicative.** Confirm the current rates, categories and
exemptions with the Central Board of Indirect Taxes and Customs or your own
adviser before relying on any figure this tool prints. Notifications are amended,
and this table states what this build assumes, not what is in force today.

## Install & run

One binary, standard library only, no external dependencies at all.

```sh
git clone https://github.com/sreenivas-sadhu-prabhakara/digital-consignment-note-bilty-lr.git
cd digital-consignment-note-bilty-lr
go build -o consignment-notes .
./consignment-notes

# or, without building first
go run .
```

Then open <http://127.0.0.1:8080>. Two flags, if you want them:

```sh
./consignment-notes -addr 127.0.0.1:9000 -data /path/to/register.json
```

Load the worked register to see the whole thing before typing anything of your
own: five notes numbered BLR/0001 to BLR/0005 running unbroken through a
delivered prepaid consignment, a to-pay load still on the road, one billed to a
third party, a cancelled note that keeps its number, and a spoiled leaf recorded
as void.

## Where data is stored

One JSON file beside the binary — `./consignment-notes-data.json` unless you
pass `-data` — readable in any text editor. It holds the office's own details,
the number series, the figures the tax position is worked out on, and every note.

It is written private to your user and rewritten in full on every change, through
a temporary file and a rename, so that a crash leaves either the old register or
the new one and never half of either. Half a register is a register with holes in
its numbering, which matters more here than in most tools.

Nothing leaves the machine. The tool binds to `127.0.0.1`, makes no outbound
network call of any kind, and has no telemetry, no analytics and no fonts or
scripts from anywhere else. It is connected to no portal, no vehicle register and
no tax system.

## Scope and accuracy

This tool prepares and keeps documents. **It is not advice, it is not a filing,
and it is connected to nothing.** It cannot tell you whether a figure is *right*
— a weight entered wrong once will be on all three copies, consistently. Check
every figure against what was actually loaded, and the tax position with your own
adviser, before the note leaves your hands.

It generates no e-way bill, computes no liability beyond the indicative figure
described above, does not price a trip, does not check a vehicle's documents, and
cannot tell you whether a consignment may lawfully move. The registration-number
checks are shape and check-character tests only: a number that passes may still
not be anybody's.

MIT licensed.
