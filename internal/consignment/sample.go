package consignment

import "time"

// A worked register, so that the whole thing — a clean run of numbers, a
// delivered consignment with a proof of delivery, a to-pay consignment still
// out, a cancelled note and a voided number — can be seen before anything real
// is typed in. The numbers run unbroken through all of it, which is the point
// the sample is there to make.

// SampleCarrier is the transport office the worked register belongs to.
func SampleCarrier() Carrier {
	return Carrier{
		Name:    "Deccan Roadlines",
		Address: "Shed 14, Goods Transport Nagar\nYeshwanthpur",
		City:    "Bengaluru",
		State:   "Karnataka",
		PIN:     "560022",
		PAN:     "AABCD1234E",
		Phone:   "080 2345 6789",
		Terms:   "Goods carried subject to the conditions of carriage printed overleaf and to the declared value stated on the face of this note.",
	}
}

// SampleBook is the series the worked register is numbered in.
func SampleBook() Book {
	return Book{Series: "BLR", Label: "Bengaluru booking office", SeqWidth: 4, StartAt: 1}
}

// SampleNotes is the worked register: five notes, numbered 1 to 5, with no gap
// and no reuse.
func SampleNotes() []Note {
	return []Note{
		sampleDelivered(),
		sampleToPay(),
		sampleThirdParty(),
		sampleCancelled(),
		sampleVoid(),
	}
}

// sampleDelivered is a prepaid consignment that was carried and signed for.
func sampleDelivered() Note {
	return Note{
		Number: Number{Series: "BLR", Seq: 1},
		Date:   MustDate(2026, time.April, 6),
		Status: StatusDelivered,
		From:   "Bengaluru",
		To:     "Hubballi",
		Consignor: Party{
			Name:    "Sahyadri Hardware Stores",
			Address: "28, Chickpet Main Road",
			City:    "Bengaluru",
			State:   "Karnataka",
			PIN:     "560053",
			GSTIN:   "29AAACS1234J1ZF",
			Phone:   "98450 11223",
		},
		Consignee: Party{
			Name:    "Malnad Building Supplies",
			Address: "Plot 9, Gokul Road Industrial Estate",
			City:    "Hubballi",
			State:   "Karnataka",
			PIN:     "580030",
			GSTIN:   "29AAFCM4567K1ZW",
			Phone:   "94480 55667",
		},
		Goods: []Goods{
			{Description: "Galvanised iron pipes, 25 mm, in bundles", Packages: 40, Packing: "Bundles", ActualGrams: 4800 * GramsPerKilogram},
			{Description: "Pipe fittings, assorted, in cartons", Packages: 12, Packing: "Cartons", ActualGrams: 360 * GramsPerKilogram},
		},
		PrivateMark:         "SHS/HBL",
		ConsignorInvoiceNo:  "SHS/26-27/0411",
		DeclaredValuePaise:  418000 * PaisePerRupee,
		ValueDeclaredByName: "Sahyadri Hardware Stores",
		Terms:               Terms{Basis: TermsPaid},
		Charges: Charges{
			Basis:          BasisPerQuintal,
			RatePaise:      185 * PaisePerRupee,
			LoadingPaise:   900 * PaisePerRupee,
			UnloadingPaise: 0,
			// The whole gross: 51.6 quintals at ₹185 is ₹9,546, and ₹900 of
			// loading on top of it. A note stamped paid whose advance is short
			// of its gross is not paid, and the register says so — which is
			// right, but not the lesson the worked example is here to teach.
			AdvancePaise: 10446 * PaisePerRupee,
		},
		Vehicle: Vehicle{Number: "KA25C4471", Kind: "1109 / 10-tyre", Owner: "Deccan Roadlines"},
		Driver:  Driver{Name: "Mahadev Kamble", LicenceNo: "KA2520180004471", Phone: "99860 33445"},
		Tax: TaxStated{
			Position:       PositionRecipientReverseCharge,
			RecipientClass: "a registered person under the Central Goods and Services Tax Act",
		},
		EWayBillNumber: "381004117762",
		TransitDays:    2,
		Delivery: Delivery{
			Date:             MustDate(2026, time.April, 8),
			ReceiverName:     "Shivanand Patil",
			ReceiverRole:     "godown keeper, for Malnad Building Supplies",
			PackagesReceived: 52,
			PackagesCounted:  true,
			Condition:        ConditionGoodOrder,
			IDShown:          "employee card",
		},
		Remarks: "Pipes loaded loose over the bundles at the consignor's request.",
	}
}

// sampleToPay is a to-pay consignment still on the road, which is the case a
// transport office most often loses money on.
func sampleToPay() Note {
	return Note{
		Number: Number{Series: "BLR", Seq: 2},
		Date:   MustDate(2026, time.April, 7),
		Status: StatusInTransit,
		From:   "Bengaluru",
		To:     "Belagavi",
		Consignor: Party{
			Name:    "Vrindavan Agro Mills",
			Address: "Survey 112, Dabaspet",
			City:    "Bengaluru Rural",
			State:   "Karnataka",
			PIN:     "562111",
			GSTIN:   "29AAGCV7788L1Z7",
		},
		Consignee: Party{
			Name:    "Shree Annapurna Traders",
			Address: "Market Yard, Shop 44",
			City:    "Belagavi",
			State:   "Karnataka",
			PIN:     "590001",
			GSTIN:   "29AAHCS2211M1Z7",
			Phone:   "90080 77889",
		},
		Goods: []Goods{
			{Description: "Groundnut oil cake, in gunny bags", Packages: 220, Packing: "Bags", ActualGrams: 11000 * GramsPerKilogram},
		},
		ConsignorInvoiceNo:  "VAM/1142",
		DeclaredValuePaise:  264000 * PaisePerRupee,
		ValueDeclaredByName: "Vrindavan Agro Mills",
		Terms:               Terms{Basis: TermsToPay},
		Charges: Charges{
			Basis:        BasisPerQuintal,
			RatePaise:    21050, // ₹210.50 per quintal
			LoadingPaise: 1650 * PaisePerRupee,
			AdvancePaise: 5000 * PaisePerRupee,
		},
		Vehicle: Vehicle{Number: "KA20B8812", Kind: "32 ft single axle"},
		Driver:  Driver{Name: "Imtiaz Shaikh", LicenceNo: "KA2020150008812", Phone: "97390 11002"},
		Tax: TaxStated{
			Position:       PositionRecipientReverseCharge,
			RecipientClass: "a registered person under the Central Goods and Services Tax Act",
		},
		EWayBillNumber: "381004118039",
		TransitDays:    2,
	}
}

// sampleThirdParty is a consignment whose freight is billed to neither end.
func sampleThirdParty() Note {
	return Note{
		Number: Number{Series: "BLR", Seq: 3},
		Date:   MustDate(2026, time.April, 9),
		Status: StatusArrived,
		From:   "Bengaluru",
		To:     "Mangaluru",
		Consignor: Party{
			Name:    "Cauvery Precision Components",
			Address: "Unit 7, Peenya Industrial Area, Phase II",
			City:    "Bengaluru",
			State:   "Karnataka",
			PIN:     "560058",
			GSTIN:   "29AADCC3344N1ZD",
		},
		Consignee: Party{
			Name:    "Netravati Marine Works",
			Address: "Baikampady Industrial Area",
			City:    "Mangaluru",
			State:   "Karnataka",
			PIN:     "575011",
			GSTIN:   "29AAECN9900P1ZR",
		},
		BillTo: Party{
			Name:    "Konkan Equipment Procurement LLP",
			Address: "3rd Floor, Trade Centre, Kadri",
			City:    "Mangaluru",
			State:   "Karnataka",
			PIN:     "575004",
			GSTIN:   "29AAIFK5566Q1ZC",
		},
		Goods: []Goods{
			{Description: "Machined steel flanges, in wooden cases", Packages: 8, Packing: "Cases", ActualGrams: 1640 * GramsPerKilogram},
		},
		ConsignorInvoiceNo:  "CPC/26-27/0088",
		DeclaredValuePaise:  592000 * PaisePerRupee,
		ValueDeclaredByName: "Cauvery Precision Components",
		Terms:               Terms{Basis: TermsBilledToThird, BilledToName: "Konkan Equipment Procurement LLP"},
		Charges: Charges{
			Basis:               BasisPerQuintal,
			RatePaise:           240 * PaisePerRupee,
			MinimumChargedGrams: 2000 * GramsPerKilogram,
			LoadingPaise:        450 * PaisePerRupee,
			DoorDeliveryPaise:   1200 * PaisePerRupee,
		},
		Vehicle:        Vehicle{Number: "KA01AJ9031", Kind: "407 / LCV"},
		Driver:         Driver{Name: "Ganesh Poojary", LicenceNo: "KA0120190009031"},
		Tax:            TaxStated{Position: PositionGTAForwardWithITC},
		EWayBillNumber: "381004118455",
		TransitDays:    1,
		Remarks:        "Charged on the two-tonne minimum for this vehicle.",
	}
}

// sampleCancelled is a note that was issued and then called off. It keeps its
// number, which is the whole reason cancellation exists as a status.
func sampleCancelled() Note {
	return Note{
		Number: Number{Series: "BLR", Seq: 4},
		Date:   MustDate(2026, time.April, 10),
		Status: StatusCancelled,
		From:   "Bengaluru",
		To:     "Davanagere",
		Consignor: Party{
			Name:  "Sahyadri Hardware Stores",
			City:  "Bengaluru",
			State: "Karnataka",
			GSTIN: "29AAACS1234J1ZF",
		},
		Consignee: Party{
			Name:  "Harihar Hardware Mart",
			City:  "Davanagere",
			State: "Karnataka",
		},
		Goods: []Goods{
			{Description: "Galvanised iron pipes, 20 mm, in bundles", Packages: 15, Packing: "Bundles", ActualGrams: 1800 * GramsPerKilogram},
		},
		Terms:   Terms{Basis: TermsPaid},
		Charges: Charges{Basis: BasisPerQuintal, RatePaise: 195 * PaisePerRupee},
		Cancellation: Cancellation{
			Date:   MustDate(2026, time.April, 10),
			Reason: "Consignor withdrew the booking before loading; goods never handed over.",
			By:     "booking counter",
		},
		Tax: TaxStated{Position: PositionUnstated},
	}
}

// sampleVoid is a number that was never used — the spoiled leaf. It is here
// because a register that cannot record one will show a gap instead, and a gap
// is what this tool exists to make impossible to ignore.
func sampleVoid() Note {
	return Note{
		Number: Number{Series: "BLR", Seq: 5},
		Date:   MustDate(2026, time.April, 11),
		Status: StatusVoid,
		Cancellation: Cancellation{
			Date:   MustDate(2026, time.April, 11),
			Reason: "Leaf spoiled at the counter — carbon misaligned, all three copies illegible. Struck through and retained in the book.",
			By:     "booking counter",
		},
		Tax: TaxStated{Position: PositionUnstated},
	}
}

// SampleRates is the figures the worked register uses: the ones this build
// ships with, with no exemption applied.
func SampleRates() Rates { return DefaultRates() }
