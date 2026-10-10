import 'vuetify/styles';
import { createVuetify } from 'vuetify';
import { VApp, VMain, VBtn, VProgressCircular } from 'vuetify/components';

export const vuetify = createVuetify({
  components: { VApp, VMain, VBtn, VProgressCircular },
  theme: {
    defaultTheme: 'writing',
    themes: {
      writing: {
        dark: false,
        colors: { primary: '#245D50', background: '#F7F8F5', surface: '#FFFFFF', error: '#C54E4E' },
      },
    },
  },
  defaults: {
    VBtn: { rounded: 'lg', elevation: 0 },
  },
});
