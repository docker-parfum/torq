import { ColumnMetaData } from "features/table/types";
import { Payment } from "./types";
import { ViewResponse } from "features/viewManagement/types";
import { FilterCategoryType, FilterInterface } from "features/sidebar/sections/filter/filter";

export const AllPaymentsColumns: Array<ColumnMetaData<Payment>> = [
  { key: "date", heading: "Date", type: "DateCell", valueType: "date" },
  {
    key: "status",
    heading: "Status",
    type: "TextCell",
    valueType: "array",
    selectOptions: [
      { label: "Succeeded", value: "SUCCEEDED" },
      { label: "In Flight", value: "IN_FLIGHT" },
      { label: "Failed", value: "FAILED" },
    ],
  },
  { key: "value", heading: "Value", type: "NumericCell", valueType: "number" },
  { key: "fee", heading: "Fee", type: "NumericCell", valueType: "number" },
  { key: "ppm", heading: "PPM", type: "NumericCell", valueType: "number" },
  { key: "isRebalance", heading: "Rebalance", type: "BooleanCell", valueType: "boolean" },
  { key: "secondsInFlight", heading: "Seconds In Flight", type: "DurationCell", valueType: "duration" },
  {
    key: "failureReason",
    heading: "Failure Reason",
    type: "TextCell",
    valueType: "array",
    selectOptions: [
      { value: "FAILURE_REASON_NONE", label: "None" },
      { value: "FAILURE_REASON_TIMEOUT", label: "Timeout" },
      { value: "FAILURE_REASON_NO_ROUTE", label: "No Route" },
      { value: "FAILURE_REASON_ERROR", label: "Error" },
      { value: "FAILURE_REASON_INCORRECT_PAYMENT_DETAILS", label: "Incorrect Payment Details" },
      { value: "FAILURE_REASON_INCORRECT_PAYMENT_AMOUNT", label: "Incorrect Payment Amount" },
      { value: "FAILURE_REASON_PAYMENT_HASH_MISMATCH", label: "Payment Hash Mismatch" },
      { value: "FAILURE_REASON_INCORRECT_PAYMENT_REQUEST", label: "Incorrect Payment Request" },
      { value: "FAILURE_REASON_UNKNOWN", label: "Unknown" },
    ],
  },
  { key: "isMpp", heading: "MPP", type: "BooleanCell", valueType: "boolean" },
  { key: "countFailedAttempts", heading: "Failed Attempts", type: "NumericCell", valueType: "number" },
  { key: "countSuccessfulAttempts", heading: "Successful Attempts", type: "NumericCell", valueType: "number" },
  { key: "destinationPubKey", heading: "Destination", type: "TextCell", valueType: "string" },
  { key: "paymentHash", heading: "Payment Hash", type: "TextCell", valueType: "string" },
  { key: "paymentPreimage", heading: "Payment Preimage", type: "TextCell", valueType: "string" },
];

const defaultColumns: Array<keyof Payment> = [
  "date",
  "status",
  "value",
  "fee",
  "ppm",
  "isRebalance",
  "secondsInFlight",
  "failureReason",
  "countFailedAttempts",
];

const sortableColumns: Array<keyof Payment> = [
  "date",
  "status",
  "value",
  "fee",
  "ppm",
  "failureReason",
  "countSuccessfulAttempts",
  "countFailedAttempts",
  "secondsInFlight",
];

export const SortablePaymentsColumns = AllPaymentsColumns.filter((column: ColumnMetaData<Payment>) =>
  sortableColumns.includes(column.key)
);

const filterableColumns: Array<keyof Payment> = [
  "date",
  "destinationPubKey",
  "status",
  "value",
  "fee",
  "ppm",
  "failureReason",
  "isRebalance",
  "isMpp",
  "countSuccessfulAttempts",
  "countFailedAttempts",
  "secondsInFlight",
  "paymentHash",
  "paymentPreimage",
];

export const FilterablePaymentsColumns = AllPaymentsColumns.filter((column: ColumnMetaData<Payment>) =>
  filterableColumns.includes(column.key)
);

export const PaymentsFilterTemplate: FilterInterface = {
  funcName: "gte",
  category: "number" as FilterCategoryType,
  parameter: 0,
  key: "value",
};

export const StatusTypeLabels = new Map<string, string>([
  ["SUCCEEDED", "Succeeded"],
  ["FAILED", "Failed"],
  ["IN_FLIGHT", "In Flight"],
]);

export const FailureReasonLabels = new Map<string, string>([
  ["FAILURE_REASON_NONE", ""],
  ["FAILURE_REASON_TIMEOUT", "Timeout"],
  ["FAILURE_REASON_NO_ROUTE", "No Route"],
  ["FAILURE_REASON_ERROR", "Error"],
  ["FAILURE_REASON_INCORRECT_PAYMENT_DETAILS", "Incorrect Payment Details"],
  ["FAILURE_REASON_INCORRECT_PAYMENT_AMOUNT", "Incorrect Payment Amount"],
  ["FAILURE_REASON_PAYMENT_HASH_MISMATCH", "Payment Hash Mismatch"],
  ["FAILURE_REASON_INCORRECT_PAYMENT_REQUEST", "Incorrect Payment Request"],
  ["FAILURE_REASON_UNKNOWN", "Unknown"],
]);

export const PaymentsSortTemplate: { key: keyof Payment; direction: "desc" | "asc" } = {
  key: "date",
  direction: "desc",
};

export const ActivePaymentsColumns: Array<ColumnMetaData<Payment>> = AllPaymentsColumns.filter((item) => {
  return defaultColumns.includes(item.key);
});

export const DefaultPaymentView: ViewResponse<Payment> = {
  page: "payments",
  dirty: true,
  view: {
    title: "Draft View",
    columns: ActivePaymentsColumns,
    sortBy: [PaymentsSortTemplate],
  },
};
