// medical-input.js - Medical case input form handler

let symptomCount = 0;
let medicationCount = 0;
let labCount = 0;
let imagingCount = 0;
let pendingCaseData = null;

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    // Sync API settings to server (same as main page does)
    syncMedicalSettings();

    // Load consent text
    loadConsent();

    // Setup event listeners
    setupEventListeners();

    // Add initial symptom, medication, and imaging fields
    addSymptom();
    addMedication();
});

function syncMedicalSettings() {
    // First, try to restore from server if localStorage is empty
    fetch('/api/config')
        .then(r => r.json())
        .then(cfg => {
            if (cfg.llm) {
                if (!localStorage.getItem('kampong_api_key') && cfg.llm.api_key) {
                    localStorage.setItem('kampong_api_key', cfg.llm.api_key);
                }
                if (!localStorage.getItem('kampong_base_url') && cfg.llm.base_url) {
                    localStorage.setItem('kampong_base_url', cfg.llm.base_url);
                }
                if (!localStorage.getItem('kampong_model') && cfg.llm.model) {
                    localStorage.setItem('kampong_model', cfg.llm.model);
                }
            }
            // Now sync whatever we have to the server
            doSyncSettings();
        })
        .catch(() => doSyncSettings());
}

function doSyncSettings() {
    const body = {};
    const apiKey = localStorage.getItem('kampong_api_key');
    const baseUrl = localStorage.getItem('kampong_base_url');
    const model = localStorage.getItem('kampong_model');

    if (apiKey) body.api_key = apiKey;
    if (baseUrl) body.base_url = baseUrl;
    if (model) body.model = model;

    if (Object.keys(body).length > 0) {
        fetch('/api/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        }).catch(err => console.error('Failed to sync settings:', err));
    }
}

function loadConsent() {
    fetch('/api/medical/consent-text')
        .then(response => response.json())
        .then(data => {
            document.getElementById('consent-text').innerHTML = data.consent_text.replace(/\n/g, '<br>');
        })
        .catch(error => {
            console.error('Error loading consent:', error);
            document.getElementById('consent-text').innerHTML = 'Error loading consent text. Please refresh the page.';
        });
}

function setupEventListeners() {
    // Consent checkbox
    document.getElementById('consent-checkbox').addEventListener('change', function() {
        document.getElementById('accept-consent-btn').disabled = !this.checked;
    });
    
    // Accept consent button
    document.getElementById('accept-consent-btn').addEventListener('click', function() {
        acceptConsent();
    });
    
    // Add buttons
    document.getElementById('add-symptom-btn').addEventListener('click', addSymptom);
    document.getElementById('add-medication-btn').addEventListener('click', addMedication);
    document.getElementById('add-lab-btn').addEventListener('click', addLab);
    document.getElementById('add-imaging-btn').addEventListener('click', addImaging);
    
    // Form submission
    document.getElementById('medical-case-form').addEventListener('submit', submitCase);
    
    // Red flag acknowledgment
    document.getElementById('acknowledge-red-flags-btn').addEventListener('click', async function() {
        document.getElementById('red-flag-warning').style.display = 'none';
        if (pendingCaseData) {
            await startDebate(pendingCaseData);
            pendingCaseData = null;
        }
    });
}

function acceptConsent() {
    const consentData = {
        accepted: true,
        timestamp: new Date().toISOString(),
        medical_professional: false // Could add a checkbox for this
    };
    
    fetch('/api/medical/consent', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(consentData)
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            // Remember the consent ID so protected endpoints can verify it.
            if (data.user_id) {
                localStorage.setItem('med_consent_user_id', data.user_id);
            }
            document.getElementById('consent-modal').style.display = 'none';
            document.getElementById('medical-case-form').style.display = 'block';
        }
    })
    .catch(error => {
        console.error('Error accepting consent:', error);
        alert('Error accepting consent. Please try again.');
    });
}

// consentHeaders returns headers carrying the informed-consent user ID.
function consentHeaders(extra) {
    const headers = Object.assign({ 'Content-Type': 'application/json' }, extra || {});
    const userId = localStorage.getItem('med_consent_user_id');
    if (userId) {
        headers['X-User-ID'] = userId;
    }
    return headers;
}

function addSymptom() {
    symptomCount++;
    const container = document.getElementById('symptoms-container');
    const symptomDiv = document.createElement('div');
    symptomDiv.className = 'symptom-entry';
    symptomDiv.id = `symptom-${symptomCount}`;
    
    symptomDiv.innerHTML = `
        <div class="symptom-header">
            <h3>Symptom ${symptomCount}</h3>
            <button type="button" class="btn-remove" onclick="removeSymptom(${symptomCount})">×</button>
        </div>
        <div class="form-grid">
            <div class="form-group">
                <label>Symptom Name</label>
                <input type="text" name="symptom-name-${symptomCount}" required>
            </div>
            <div class="form-group">
                <label>Severity (1-10)</label>
                <input type="number" name="symptom-severity-${symptomCount}" min="1" max="10" value="5">
            </div>
            <div class="form-group">
                <label>Onset</label>
                <select name="symptom-onset-${symptomCount}">
                    <option value="acute">Acute</option>
                    <option value="subacute">Subacute</option>
                    <option value="chronic">Chronic</option>
                </select>
            </div>
            <div class="form-group">
                <label>Duration</label>
                <input type="text" name="symptom-duration-${symptomCount}" placeholder="e.g., 3 days, 2 weeks">
            </div>
            <div class="form-group">
                <label>Location</label>
                <input type="text" name="symptom-location-${symptomCount}" placeholder="e.g., chest, head, abdomen">
            </div>
            <div class="form-group">
                <label>Aggravating Factors</label>
                <input type="text" name="symptom-aggravating-${symptomCount}" placeholder="What makes it worse?">
            </div>
            <div class="form-group">
                <label>Alleviating Factors</label>
                <input type="text" name="symptom-alleviating-${symptomCount}" placeholder="What makes it better?">
            </div>
            <div class="form-group">
                <label>Associated Symptoms</label>
                <input type="text" name="symptom-associated-${symptomCount}" placeholder="Other related symptoms">
            </div>
        </div>
    `;
    
    container.appendChild(symptomDiv);
}

function removeSymptom(id) {
    const element = document.getElementById(`symptom-${id}`);
    if (element) {
        element.remove();
    }
}

function addMedication() {
    medicationCount++;
    const container = document.getElementById('medications-container');
    const medDiv = document.createElement('div');
    medDiv.className = 'medication-entry';
    medDiv.id = `medication-${medicationCount}`;
    
    medDiv.innerHTML = `
        <div class="medication-header">
            <h3>Medication ${medicationCount}</h3>
            <button type="button" class="btn-remove" onclick="removeMedication(${medicationCount})">×</button>
        </div>
        <div class="form-grid">
            <div class="form-group">
                <label>Medication Name</label>
                <input type="text" name="med-name-${medicationCount}" required>
            </div>
            <div class="form-group">
                <label>Dose</label>
                <input type="text" name="med-dose-${medicationCount}" placeholder="e.g., 500mg">
            </div>
            <div class="form-group">
                <label>Frequency</label>
                <input type="text" name="med-frequency-${medicationCount}" placeholder="e.g., twice daily">
            </div>
            <div class="form-group">
                <label>Route</label>
                <select name="med-route-${medicationCount}">
                    <option value="oral">Oral</option>
                    <option value="IV">IV</option>
                    <option value="IM">IM</option>
                    <option value="topical">Topical</option>
                    <option value="other">Other</option>
                </select>
            </div>
        </div>
    `;
    
    container.appendChild(medDiv);
}

function removeMedication(id) {
    const element = document.getElementById(`medication-${id}`);
    if (element) {
        element.remove();
    }
}

function addLab() {
    labCount++;
    const container = document.getElementById('labs-container');
    const labDiv = document.createElement('div');
    labDiv.className = 'lab-entry';
    labDiv.id = `lab-${labCount}`;
    
    labDiv.innerHTML = `
        <div class="lab-header">
            <h3>Lab Result ${labCount}</h3>
            <button type="button" class="btn-remove" onclick="removeLab(${labCount})">×</button>
        </div>
        <div class="form-grid">
            <div class="form-group">
                <label>Test Name</label>
                <input type="text" name="lab-name-${labCount}" required>
            </div>
            <div class="form-group">
                <label>Value</label>
                <input type="number" name="lab-value-${labCount}" step="0.01" required>
            </div>
            <div class="form-group">
                <label>Unit</label>
                <input type="text" name="lab-unit-${labCount}" placeholder="e.g., mg/dL, mmol/L">
            </div>
            <div class="form-group">
                <label>Reference Min</label>
                <input type="number" name="lab-ref-min-${labCount}" step="0.01">
            </div>
            <div class="form-group">
                <label>Reference Max</label>
                <input type="number" name="lab-ref-max-${labCount}" step="0.01">
            </div>
        </div>
    `;
    
    container.appendChild(labDiv);
}

function removeLab(id) {
    const element = document.getElementById(`lab-${id}`);
    if (element) {
        element.remove();
    }
}

function addImaging() {
    imagingCount++;
    const container = document.getElementById('imaging-container');
    const imgDiv = document.createElement('div');
    imgDiv.className = 'imaging-entry';
    imgDiv.id = `imaging-${imagingCount}`;

    imgDiv.innerHTML = `
        <div class="imaging-header">
            <h3>Imaging Study ${imagingCount}</h3>
            <button type="button" class="btn-remove" onclick="removeImaging(${imagingCount})">×</button>
        </div>
        <div class="form-grid">
            <div class="form-group">
                <label>Modality</label>
                <select name="img-modality-${imagingCount}">
                    <option value="Lab Report">Lab Report</option>
                    <option value="X-ray">X-ray</option>
                    <option value="CT">CT Scan</option>
                    <option value="MRI">MRI</option>
                    <option value="Ultrasound">Ultrasound</option>
                    <option value="ECG">ECG / EKG</option>
                    <option value="PET">PET Scan</option>
                    <option value="Mammogram">Mammogram</option>
                    <option value="Fluoroscopy">Fluoroscopy</option>
                    <option value="Chart">Chart / Graph</option>
                    <option value="Photo">Clinical Photo</option>
                    <option value="Other">Other</option>
                </select>
            </div>
            <div class="form-group">
                <label>Body Part / Region</label>
                <input type="text" name="img-bodypart-${imagingCount}" placeholder="e.g., Chest, Knee, Abdomen, 12-lead">
            </div>
            <div class="form-group">
                <label>Date of Study</label>
                <input type="date" name="img-date-${imagingCount}">
            </div>
            <div class="form-group">
                <label>Upload Image</label>
                <input type="file" name="img-file-${imagingCount}" accept="image/jpeg,image/png,image/webp,image/bmp,image/gif" class="img-file-input" onchange="previewImaging(${imagingCount}, this)">
            </div>
        </div>
        <div class="form-group">
            <label>Text Finding / Report (if no image)</label>
            <textarea name="img-finding-${imagingCount}" placeholder="e.g., PA chest X-ray shows right lower lobe consolidation..."></textarea>
        </div>
        <div class="imaging-preview" id="img-preview-${imagingCount}" style="display:none;">
            <img id="img-preview-thumb-${imagingCount}" src="" alt="Preview">
            <div class="imaging-analysis-status" id="img-analysis-status-${imagingCount}"></div>
        </div>
    `;

    container.appendChild(imgDiv);
}

function removeImaging(id) {
    const element = document.getElementById(`imaging-${id}`);
    if (element) {
        element.remove();
    }
}

function previewImaging(id, input) {
    const preview = document.getElementById(`img-preview-${id}`);
    const thumb = document.getElementById(`img-preview-thumb-${id}`);

    if (input.files && input.files[0]) {
        const file = input.files[0];
        if (file.size > 10 * 1024 * 1024) {
            alert('Image must be under 10MB');
            input.value = '';
            return;
        }
        const reader = new FileReader();
        reader.onload = function(e) {
            thumb.src = e.target.result;
            preview.style.display = 'block';
        };
        reader.readAsDataURL(file);
    }
}

async function uploadImagingFile(id, file) {
    const formData = new FormData();
    formData.append('file', file);

    try {
        const response = await fetch('/api/upload', {
            method: 'POST',
            body: formData
        });
        const data = await response.json();
        return data;
    } catch (err) {
        console.error(`Error uploading imaging file ${id}:`, err);
        return null;
    }
}

async function submitCase(event) {
    event.preventDefault();

    // Collect form data
    const caseData = await collectCaseData();

    // Check for red flags (client-side preliminary check)
    const redFlags = checkRedFlags(caseData);

    if (redFlags.length > 0) {
        pendingCaseData = caseData;
        showRedFlagWarning(redFlags);
    } else {
        await startDebate(caseData);
    }
}

async function collectCaseData() {
    // Patient info
    const caseData = {
        patient_info: {
            age: parseInt(document.getElementById('patient-age').value),
            sex: document.getElementById('patient-sex').value,
            weight: parseFloat(document.getElementById('patient-weight').value) || 0,
            height: parseFloat(document.getElementById('patient-height').value) || 0,
            smoking_status: document.getElementById('patient-smoking').value,
            alcohol_use: document.getElementById('patient-alcohol').value
        },
        symptoms: collectSymptoms(),
        vital_signs: {
            temperature: parseFloat(document.getElementById('vital-temp').value) || 0,
            heart_rate: parseInt(document.getElementById('vital-hr').value) || 0,
            blood_pressure: document.getElementById('vital-bp').value,
            respiratory_rate: parseInt(document.getElementById('vital-rr').value) || 0,
            spo2: parseInt(document.getElementById('vital-spo2').value) || 0,
            pain_score: parseInt(document.getElementById('vital-pain').value) || 0
        },
        medical_history: {
            past_conditions: document.getElementById('history-conditions').value.split('\n').filter(s => s.trim()),
            surgeries: document.getElementById('history-surgeries').value.split('\n').filter(s => s.trim()),
            family_history: document.getElementById('history-family').value.split('\n').filter(s => s.trim())
        },
        current_meds: collectMedications(),
        allergies: document.getElementById('allergies').value.split('\n').filter(s => s.trim()),
        lab_results: collectLabs(),
        imaging_results: await collectImaging(),
        // extractedLabResults is populated by collectImaging() when Lab Report images are processed
        debate_mode: document.querySelector('input[name="debate-mode"]:checked').value
    };

    // Merge extracted lab results from images into lab_results
    if (extractedLabResults.length > 0) {
        caseData.lab_results = caseData.lab_results.concat(extractedLabResults);
    }

    // Calculate BMI
    if (caseData.patient_info.weight > 0 && caseData.patient_info.height > 0) {
        const heightM = caseData.patient_info.height / 100;
        caseData.patient_info.bmi = caseData.patient_info.weight / (heightM * heightM);
    }

    return caseData;
}

function collectSymptoms() {
    const symptoms = [];
    const symptomEntries = document.querySelectorAll('.symptom-entry');
    
    symptomEntries.forEach((entry, index) => {
        const id = entry.id.split('-')[1];
        const name = entry.querySelector(`[name="symptom-name-${id}"]`).value;
        
        if (name.trim()) {
            symptoms.push({
                name: name,
                severity: parseInt(entry.querySelector(`[name="symptom-severity-${id}"]`).value) || 5,
                onset: entry.querySelector(`[name="symptom-onset-${id}"]`).value,
                duration: entry.querySelector(`[name="symptom-duration-${id}"]`).value,
                location: entry.querySelector(`[name="symptom-location-${id}"]`).value,
                aggravating: entry.querySelector(`[name="symptom-aggravating-${id}"]`).value,
                alleviating: entry.querySelector(`[name="symptom-alleviating-${id}"]`).value,
                associated: entry.querySelector(`[name="symptom-associated-${id}"]`).value
            });
        }
    });
    
    return symptoms;
}

function collectMedications() {
    const medications = [];
    const medEntries = document.querySelectorAll('.medication-entry');
    
    medEntries.forEach((entry) => {
        const id = entry.id.split('-')[1];
        const name = entry.querySelector(`[name="med-name-${id}"]`).value;
        
        if (name.trim()) {
            medications.push({
                name: name,
                dose: entry.querySelector(`[name="med-dose-${id}"]`).value,
                frequency: entry.querySelector(`[name="med-frequency-${id}"]`).value,
                route: entry.querySelector(`[name="med-route-${id}"]`).value
            });
        }
    });
    
    return medications;
}

function collectLabs() {
    const labs = [];
    const labEntries = document.querySelectorAll('.lab-entry');
    
    labEntries.forEach((entry) => {
        const id = entry.id.split('-')[1];
        const name = entry.querySelector(`[name="lab-name-${id}"]`).value;
        
        if (name.trim()) {
            const value = parseFloat(entry.querySelector(`[name="lab-value-${id}"]`).value);
            const refMin = parseFloat(entry.querySelector(`[name="lab-ref-min-${id}"]`).value) || 0;
            const refMax = parseFloat(entry.querySelector(`[name="lab-ref-max-${id}"]`).value) || 0;
            
            let flag = 'normal';
            if (value < refMin) flag = 'low';
            else if (value > refMax) flag = 'high';
            
            labs.push({
                test_name: name,
                value: value,
                unit: entry.querySelector(`[name="lab-unit-${id}"]`).value,
                reference_min: refMin,
                reference_max: refMax,
                flag: flag
            });
        }
    });
    
    return labs;
}

let extractedLabResults = [];

async function collectImaging() {
    const imaging = [];
    extractedLabResults = [];
    const imagingEntries = document.querySelectorAll('.imaging-entry');

    for (const entry of imagingEntries) {
        const id = entry.id.split('-')[1];
        const modality = entry.querySelector(`[name="img-modality-${id}"]`).value;
        const bodyPart = entry.querySelector(`[name="img-bodypart-${id}"]`).value;
        const date = entry.querySelector(`[name="img-date-${id}"]`).value;
        const finding = entry.querySelector(`[name="img-finding-${id}"]`).value;
        const fileInput = entry.querySelector(`[name="img-file-${id}"]`);

        if (!modality && !bodyPart && !finding && (!fileInput || !fileInput.files || !fileInput.files[0])) {
            continue;
        }

        const result = {
            modality: modality,
            body_part: bodyPart,
            finding: finding,
            date: date,
            image_path: ''
        };

        // Handle file upload
        if (fileInput && fileInput.files && fileInput.files[0]) {
            const statusEl = document.getElementById(`img-analysis-status-${id}`);

            if (modality === 'Lab Report') {
                // Lab Report: use specialized extraction endpoint
                if (statusEl) statusEl.textContent = 'Extracting lab values from image...';

                const labData = await extractLabFromImage(fileInput.files[0]);
                if (labData && labData.lab_results && labData.lab_results.length > 0) {
                    extractedLabResults = extractedLabResults.concat(labData.lab_results);
                    result.finding = (result.finding ? result.finding + '\n\n' : '') +
                        `[AI Lab Extraction] Extracted ${labData.count} lab values from image`;
                    if (statusEl) statusEl.textContent = `✓ Extracted ${labData.count} lab values`;

                    // Also upload the file for storage
                    const uploadResult = await uploadImagingFile(id, fileInput.files[0]);
                    if (uploadResult && uploadResult.file_path) {
                        result.image_path = uploadResult.file_path;
                    }
                } else {
                    if (statusEl) statusEl.textContent = '✗ Could not extract lab values — try a clearer image';
                    // Fallback: upload as regular image
                    const uploadResult = await uploadImagingFile(id, fileInput.files[0]);
                    if (uploadResult && uploadResult.file_path) {
                        result.image_path = uploadResult.file_path;
                        if (uploadResult.analysis) {
                            result.finding = (result.finding ? result.finding + '\n\n' : '') +
                                '[AI Image Analysis]\n' + uploadResult.analysis;
                        }
                    }
                }
            } else {
                // Non-lab image: regular upload + analysis
                if (statusEl) statusEl.textContent = 'Uploading image...';

                const uploadResult = await uploadImagingFile(id, fileInput.files[0]);
                if (uploadResult && uploadResult.file_path) {
                    result.image_path = uploadResult.file_path;
                    if (uploadResult.analysis) {
                        result.finding = (result.finding ? result.finding + '\n\n' : '') +
                            '[AI Image Analysis]\n' + uploadResult.analysis;
                        if (statusEl) statusEl.textContent = '✓ Image analyzed';
                    } else {
                        if (statusEl) statusEl.textContent = '✓ Image uploaded';
                    }
                } else {
                    if (statusEl) statusEl.textContent = '✗ Upload failed';
                }
            }
        }

        imaging.push(result);
    }

    return imaging;
}

async function extractLabFromImage(file) {
    const formData = new FormData();
    formData.append('file', file);

    try {
        const response = await fetch('/api/medical/extract-lab-image', {
            method: 'POST',
            body: formData
        });
        const data = await response.json();
        return data;
    } catch (err) {
        console.error('Error extracting lab values:', err);
        return null;
    }
}

function checkRedFlags(caseData) {
    const redFlags = [];
    
    // Check vital signs
    if (caseData.vital_signs.temperature > 39) {
        redFlags.push({
            symptom: 'High fever',
            severity: 'urgent',
            description: 'Temperature > 39°C',
            action: 'Consider sepsis workup, blood cultures'
        });
    }
    
    if (caseData.vital_signs.heart_rate > 120 || caseData.vital_signs.heart_rate < 40) {
        redFlags.push({
            symptom: 'Abnormal heart rate',
            severity: 'urgent',
            description: 'Heart rate outside normal range',
            action: 'ECG, cardiac monitoring'
        });
    }
    
    if (caseData.vital_signs.spo2 > 0 && caseData.vital_signs.spo2 < 90) {
        redFlags.push({
            symptom: 'Low oxygen saturation',
            severity: 'critical',
            description: 'SpO2 < 90%',
            action: 'Immediate oxygen therapy, consider emergency services'
        });
    }
    
    // Check symptoms for red flags
    const redFlagSymptoms = [
        'chest pain', 'chest pressure', 'chest tightness',
        'facial droop', 'arm weakness', 'speech difficulty',
        'worst headache', 'thunderclap headache',
        'severe shortness of breath', 'coughing blood',
        'severe abdominal pain', 'vomiting blood'
    ];
    
    caseData.symptoms.forEach(symptom => {
        const symptomLower = symptom.name.toLowerCase();
        redFlagSymptoms.forEach(redFlag => {
            if (symptomLower.includes(redFlag)) {
                redFlags.push({
                    symptom: symptom.name,
                    severity: 'critical',
                    description: 'Potential emergency symptom',
                    action: 'Seek immediate medical attention'
                });
            }
        });
    });
    
    return redFlags;
}

function showRedFlagWarning(redFlags) {
    const warningDiv = document.getElementById('red-flag-warning');
    const listDiv = document.getElementById('red-flag-list');
    
    listDiv.innerHTML = '';
    redFlags.forEach(flag => {
        const flagDiv = document.createElement('div');
        flagDiv.className = `red-flag-item severity-${flag.severity}`;
        flagDiv.innerHTML = `
            <h4>${flag.symptom}</h4>
            <p><strong>Severity:</strong> ${flag.severity}</p>
            <p><strong>Description:</strong> ${flag.description}</p>
            <p><strong>Recommended Action:</strong> ${flag.action}</p>
        `;
        listDiv.appendChild(flagDiv);
    });
    
    warningDiv.style.display = 'block';
}

async function startDebate(caseData) {
    if (!caseData) {
        caseData = await collectCaseData();
    }

    try {
        const response = await fetch('/api/medical/debate/start', {
            method: 'POST',
            headers: consentHeaders(),
            body: JSON.stringify(caseData)
        });
        const data = await response.json();
        if (data.debate_id) {
            // Store medical knowledge for display in the arena
            if (data.medical_knowledge) {
                sessionStorage.setItem('med_knowledge_' + data.debate_id, data.medical_knowledge);
            }
            if (data.red_flags && data.red_flags.length > 0) {
                sessionStorage.setItem('med_redflags_' + data.debate_id, JSON.stringify(data.red_flags));
            }
            window.location.href = `/?debate_id=${data.debate_id}&mode=medical`;
        } else {
            alert('Error starting debate: ' + (data.error || 'Unknown error'));
        }
    } catch (error) {
        console.error('Error starting debate:', error);
        alert('Error starting debate. Please try again.');
    }
}
