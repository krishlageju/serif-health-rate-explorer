// Readable labels for codes that appear in rate files.

// CMS place-of-service codes most common in rate files.
const PLACE_OF_SERVICE: Record<string, string> = {
  '02': 'Telehealth (not in home)',
  '10': 'Telehealth in home',
  '11': 'Office',
  '12': 'Home',
  '19': 'Off-campus outpatient hospital',
  '20': 'Urgent care',
  '21': 'Inpatient hospital',
  '22': 'On-campus outpatient hospital',
  '23': 'Emergency room',
  '24': 'Ambulatory surgical center',
  '31': 'Skilled nursing facility',
  '32': 'Nursing facility',
  '49': 'Independent clinic',
  '50': 'Federally qualified health center',
  '65': 'Dialysis facility',
  '81': 'Independent laboratory',
  CSTM: 'Custom place of service',
}

/** Readable label for a CMS place-of-service code. */
export const placeOfService = (code: string) => PLACE_OF_SERVICE[code] ?? `Place of service ${code}`

// UB-04 revenue code categories seen most often in facility rate files.
const REVENUE_CODES: Record<string, string> = {
  '250': 'Pharmacy',
  '260': 'IV therapy',
  '270': 'Medical/surgical supplies',
  '300': 'Laboratory',
  '301': 'Laboratory: chemistry',
  '302': 'Laboratory: immunology',
  '305': 'Laboratory: hematology',
  '306': 'Laboratory: bacteriology',
  '310': 'Pathology lab',
  '320': 'Diagnostic radiology',
  '324': 'Radiology: chest X-ray',
  '333': 'Radiation therapy',
  '340': 'Nuclear medicine',
  '350': 'CT scan',
  '360': 'Operating room services',
  '361': 'Operating room: minor surgery',
  '370': 'Anesthesia',
  '400': 'Imaging services',
  '401': 'Imaging: mammography',
  '402': 'Imaging: ultrasound',
  '420': 'Physical therapy',
  '430': 'Occupational therapy',
  '440': 'Speech therapy',
  '450': 'Emergency room',
  '456': 'Urgent care',
  '460': 'Pulmonary function',
  '480': 'Cardiology',
  '490': 'Ambulatory surgical care',
  '510': 'Clinic',
  '520': 'Free-standing clinic',
  '521': 'Free-standing clinic: rural health',
  '529': 'Free-standing clinic: other',
  '610': 'MRI',
  '636': 'Drugs requiring detailed coding',
  '710': 'Recovery room',
  '730': 'EKG/ECG',
  '750': 'Gastrointestinal services',
  '760': 'Treatment/observation room',
  '820': 'Hemodialysis',
  '900': 'Behavioral health',
  '914': 'Behavioral health: individual therapy',
  '915': 'Behavioral health: group therapy',
  '940': 'Other therapeutic services',
}

/** Readable label for a revenue code, if it is in the lookup table. */
export const revenueCodeName = (code: string) => REVENUE_CODES[code.replace(/^0+/, '')]
