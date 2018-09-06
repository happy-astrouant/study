package openEHR

type Shared struct {
	Category string `json:"/category"`
	ID       string `json:"/composer|identifier"`
	Name     string `json:"/composer|name"`
	Timstamp string `json:"/context/start_time"`
	Language string `json:"/language"`
}

type PersonalData struct {
	Shared
	BirthDate  string `json:"/content[openEHR-DEMOGRAPHIC-PERSON.person.v1]/details[openEHR-DEMOGRAPHIC-ITEM_TREE.person_details.v1.0.0]/items[at0010]"`
	Gender     string `json:"/content[openEHR-DEMOGRAPHIC-PERSON.person.v1]/details[openEHR-DEMOGRAPHIC-ITEM_TREE.person_details.v1.0.0]/items[at0017]"`
	FirstName  string `json:"/content[openEHR-DEMOGRAPHIC-PERSON.person.v1]/identities[openEHR-DEMOGRAPHIC-PARTY_IDENTITY.person_name.v1]/details[at0001]/items[at0002]"`
	FamilyName string `json:"/content[openEHR-DEMOGRAPHIC-PERSON.person.v1]/identities[openEHR-DEMOGRAPHIC-PARTY_IDENTITY.person_name.v1]/details[at0001]/items[at0003]"`
}

type VitalSigns struct {
	Shared
	Weight
	Glucose
	BloodPressure
}

type Weight struct {
	Measure string `json:"/content[openEHR-EHR-COMPOSITION.encounter.v1]/context/other_context/items[openEHR-EHR-OBSERVATION.body_weight.v2]/data[at0002]/events[at0003]:0/data[at0001]/items[at0004],omitempty"`
	Ts      string `json:"/content[openEHR-EHR-COMPOSITION.encounter.v1]/context/other_context/items[openEHR-EHR-OBSERVATION.body_weight.v2]/data[at0002]/events[at0003]:0/time,omitempty"`
}

type Glucose struct {
	Measure string `json:"/content[openEHR-EHR-COMPOSITION.encounter.v1]/context/other_context/items[openEHR-EHR-OBSERVATION.lab_test-blood_glucose.v1]/data[at0001]/events[at0002]:0/data[at0003]/items[at0078.2],omitempty"`
	Ts      string `json:"/content[openEHR-EHR-COMPOSITION.encounter.v1]/context/other_context/items[openEHR-EHR-OBSERVATION.lab_test-blood_glucose.v1]/data[at0001]/events[at0002]:0/time,omitempty"`
}

type BloodPressure struct {
	Diastolyc string `json:"/content[openEHR-EHR-COMPOSITION.encounter.v1]/context/other_context/items[openEHR-EHR-OBSERVATION.blood_pressure.v1]/data[at0001]/events[at0006]:0/data[at0003]/items[at0004],omitempty"`
	Systolic  string `json:"/content[openEHR-EHR-COMPOSITION.encounter.v1]/context/other_context/items[openEHR-EHR-OBSERVATION.blood_pressure.v1]/data[at0001]/events[at0006]:0/data[at0003]/items[at0005],omitempty"`
	Ts        string `json:"/content[openEHR-EHR-COMPOSITION.encounter.v1]/context/other_context/items[openEHR-EHR-OBSERVATION.blood_pressure.v1]/data[at0001]/events[at0006]:0/time,omitempty"`
}