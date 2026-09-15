import { createTheme } from '@mui/material/styles';

export const theme = createTheme({
  palette: {
    mode: 'dark',
    background: { default: '#14171c', paper: '#1b1f26' },
    primary: { main: '#c8a24a' },
    success: { main: '#5fb87a' },
    error: { main: '#d1697a' },
    divider: '#2c323c',
    text: { primary: '#e6e8ec', secondary: '#9aa3b0' },
  },
  shape: { borderRadius: 6 },
  typography: {
    fontSize: 13,
    fontFamily: '"Inter", system-ui, -apple-system, "Segoe UI", sans-serif',
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        // Digits line up across table columns only with tabular figures.
        body: { fontVariantNumeric: 'tabular-nums' },
      },
    },
    MuiTableCell: { styleOverrides: { root: { borderColor: '#2c323c' } } },
  },
});
