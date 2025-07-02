/**
 * Formats a number for display, showing one decimal place only if necessary.
 * @param {number|string} value - The number to format.
 * @returns {string} The formatted number as a string.
 */
export const formatNumberForDisplay = (value) => {
  const floatValue = parseFloat(value);
  if (isNaN(floatValue)) return "0";

  const formatted = floatValue.toFixed(2);
  return parseFloat(formatted).toString();
};

/**
 * Rounds a number to one decimal place for backend operations.
 * @param {number|string} value - The number to round.
 * @returns {number} The rounded number.
 */
export const formatForBackend = (value) => {
  return parseFloat(parseFloat(value).toFixed(2));
};

export const formatNumericInput = (value) => {
  // Replace commas with dots
  let formattedValue = value.replace(",", ".");
  // Remove any non-numeric characters except for the decimal point
  formattedValue = formattedValue.replace(/[^0-9.]/g, "");
  // Ensure only one decimal point
  const parts = formattedValue.split(".");
  if (parts.length > 2) {
    formattedValue = parts[0] + "." + parts.slice(1).join("");
  }
  return formattedValue;
};
