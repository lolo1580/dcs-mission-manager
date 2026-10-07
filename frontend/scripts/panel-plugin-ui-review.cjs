async (page) => {
  page.setDefaultTimeout(5000);
  await page.reload();
  await page.getByRole('button',{name:'Paramètres',exact:true}).click();
  await page.getByRole('button',{name:'Panneaux',exact:true}).click();
  await page.getByRole('heading',{name:/Plugin panels DCS Manager/}).waitFor();
  if (await page.getByText(/vJoy/).count()) throw new Error('vJoy still visible');
  await page.getByText('FA-18C_hornet · 2 impulsions acceptées (mouvement à vérifier en cockpit)',{exact:true}).waitFor();
  await page.getByRole('button',{name:'PZ70 Multi Panel',exact:true}).click();
  await page.getByRole('button',{name:/^Trim de profondeur ↔/}).click();
  await page.getByRole('combobox',{name:'Commande DCS-BIOS',exact:true}).selectOption('DCSM_PITCH_TRIM');
  if (await page.getByRole('combobox',{name:'Interface',exact:true}).inputValue() !== 'variable_step') throw new Error('Missing trim interface');
  await page.getByRole('button',{name:'Associer',exact:true}).click();
  await page.getByRole('button',{name:/^Trim de profondeur ↔ DCSM_PITCH_TRIM/}).waitFor();
  return 'PASS: plugin status, no vJoy, trim assignment';
}
