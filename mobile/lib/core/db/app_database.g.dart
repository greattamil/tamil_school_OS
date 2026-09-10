// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_database.dart';

// ignore_for_file: type=lint
class $LocalAttendanceEntriesTable extends LocalAttendanceEntries
    with TableInfo<$LocalAttendanceEntriesTable, LocalAttendanceEntry> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $LocalAttendanceEntriesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _enrollmentIdMeta = const VerificationMeta(
    'enrollmentId',
  );
  @override
  late final GeneratedColumn<String> enrollmentId = GeneratedColumn<String>(
    'enrollment_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _dateMeta = const VerificationMeta('date');
  @override
  late final GeneratedColumn<DateTime> date = GeneratedColumn<DateTime>(
    'date',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _sectionIdMeta = const VerificationMeta(
    'sectionId',
  );
  @override
  late final GeneratedColumn<String> sectionId = GeneratedColumn<String>(
    'section_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _studentNameMeta = const VerificationMeta(
    'studentName',
  );
  @override
  late final GeneratedColumn<String> studentName = GeneratedColumn<String>(
    'student_name',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _rollNumberMeta = const VerificationMeta(
    'rollNumber',
  );
  @override
  late final GeneratedColumn<String> rollNumber = GeneratedColumn<String>(
    'roll_number',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _statusMeta = const VerificationMeta('status');
  @override
  late final GeneratedColumn<String> status = GeneratedColumn<String>(
    'status',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _reasonMeta = const VerificationMeta('reason');
  @override
  late final GeneratedColumn<String> reason = GeneratedColumn<String>(
    'reason',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _serverRevisionMeta = const VerificationMeta(
    'serverRevision',
  );
  @override
  late final GeneratedColumn<int> serverRevision = GeneratedColumn<int>(
    'server_revision',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultValue: const Constant(0),
  );
  static const VerificationMeta _syncStatusMeta = const VerificationMeta(
    'syncStatus',
  );
  @override
  late final GeneratedColumn<String> syncStatus = GeneratedColumn<String>(
    'sync_status',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant('synced'),
  );
  @override
  List<GeneratedColumn> get $columns => [
    enrollmentId,
    date,
    sectionId,
    studentName,
    rollNumber,
    status,
    reason,
    serverRevision,
    syncStatus,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'local_attendance_entries';
  @override
  VerificationContext validateIntegrity(
    Insertable<LocalAttendanceEntry> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('enrollment_id')) {
      context.handle(
        _enrollmentIdMeta,
        enrollmentId.isAcceptableOrUnknown(
          data['enrollment_id']!,
          _enrollmentIdMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_enrollmentIdMeta);
    }
    if (data.containsKey('date')) {
      context.handle(
        _dateMeta,
        date.isAcceptableOrUnknown(data['date']!, _dateMeta),
      );
    } else if (isInserting) {
      context.missing(_dateMeta);
    }
    if (data.containsKey('section_id')) {
      context.handle(
        _sectionIdMeta,
        sectionId.isAcceptableOrUnknown(data['section_id']!, _sectionIdMeta),
      );
    } else if (isInserting) {
      context.missing(_sectionIdMeta);
    }
    if (data.containsKey('student_name')) {
      context.handle(
        _studentNameMeta,
        studentName.isAcceptableOrUnknown(
          data['student_name']!,
          _studentNameMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_studentNameMeta);
    }
    if (data.containsKey('roll_number')) {
      context.handle(
        _rollNumberMeta,
        rollNumber.isAcceptableOrUnknown(data['roll_number']!, _rollNumberMeta),
      );
    }
    if (data.containsKey('status')) {
      context.handle(
        _statusMeta,
        status.isAcceptableOrUnknown(data['status']!, _statusMeta),
      );
    }
    if (data.containsKey('reason')) {
      context.handle(
        _reasonMeta,
        reason.isAcceptableOrUnknown(data['reason']!, _reasonMeta),
      );
    }
    if (data.containsKey('server_revision')) {
      context.handle(
        _serverRevisionMeta,
        serverRevision.isAcceptableOrUnknown(
          data['server_revision']!,
          _serverRevisionMeta,
        ),
      );
    }
    if (data.containsKey('sync_status')) {
      context.handle(
        _syncStatusMeta,
        syncStatus.isAcceptableOrUnknown(data['sync_status']!, _syncStatusMeta),
      );
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {enrollmentId, date};
  @override
  LocalAttendanceEntry map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return LocalAttendanceEntry(
      enrollmentId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}enrollment_id'],
      )!,
      date: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}date'],
      )!,
      sectionId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}section_id'],
      )!,
      studentName: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}student_name'],
      )!,
      rollNumber: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}roll_number'],
      ),
      status: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}status'],
      ),
      reason: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}reason'],
      ),
      serverRevision: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}server_revision'],
      )!,
      syncStatus: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}sync_status'],
      )!,
    );
  }

  @override
  $LocalAttendanceEntriesTable createAlias(String alias) {
    return $LocalAttendanceEntriesTable(attachedDatabase, alias);
  }
}

class LocalAttendanceEntry extends DataClass
    implements Insertable<LocalAttendanceEntry> {
  final String enrollmentId;
  final DateTime date;
  final String sectionId;
  final String studentName;
  final String? rollNumber;
  final String? status;
  final String? reason;
  final int serverRevision;
  final String syncStatus;
  const LocalAttendanceEntry({
    required this.enrollmentId,
    required this.date,
    required this.sectionId,
    required this.studentName,
    this.rollNumber,
    this.status,
    this.reason,
    required this.serverRevision,
    required this.syncStatus,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['enrollment_id'] = Variable<String>(enrollmentId);
    map['date'] = Variable<DateTime>(date);
    map['section_id'] = Variable<String>(sectionId);
    map['student_name'] = Variable<String>(studentName);
    if (!nullToAbsent || rollNumber != null) {
      map['roll_number'] = Variable<String>(rollNumber);
    }
    if (!nullToAbsent || status != null) {
      map['status'] = Variable<String>(status);
    }
    if (!nullToAbsent || reason != null) {
      map['reason'] = Variable<String>(reason);
    }
    map['server_revision'] = Variable<int>(serverRevision);
    map['sync_status'] = Variable<String>(syncStatus);
    return map;
  }

  LocalAttendanceEntriesCompanion toCompanion(bool nullToAbsent) {
    return LocalAttendanceEntriesCompanion(
      enrollmentId: Value(enrollmentId),
      date: Value(date),
      sectionId: Value(sectionId),
      studentName: Value(studentName),
      rollNumber: rollNumber == null && nullToAbsent
          ? const Value.absent()
          : Value(rollNumber),
      status: status == null && nullToAbsent
          ? const Value.absent()
          : Value(status),
      reason: reason == null && nullToAbsent
          ? const Value.absent()
          : Value(reason),
      serverRevision: Value(serverRevision),
      syncStatus: Value(syncStatus),
    );
  }

  factory LocalAttendanceEntry.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return LocalAttendanceEntry(
      enrollmentId: serializer.fromJson<String>(json['enrollmentId']),
      date: serializer.fromJson<DateTime>(json['date']),
      sectionId: serializer.fromJson<String>(json['sectionId']),
      studentName: serializer.fromJson<String>(json['studentName']),
      rollNumber: serializer.fromJson<String?>(json['rollNumber']),
      status: serializer.fromJson<String?>(json['status']),
      reason: serializer.fromJson<String?>(json['reason']),
      serverRevision: serializer.fromJson<int>(json['serverRevision']),
      syncStatus: serializer.fromJson<String>(json['syncStatus']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'enrollmentId': serializer.toJson<String>(enrollmentId),
      'date': serializer.toJson<DateTime>(date),
      'sectionId': serializer.toJson<String>(sectionId),
      'studentName': serializer.toJson<String>(studentName),
      'rollNumber': serializer.toJson<String?>(rollNumber),
      'status': serializer.toJson<String?>(status),
      'reason': serializer.toJson<String?>(reason),
      'serverRevision': serializer.toJson<int>(serverRevision),
      'syncStatus': serializer.toJson<String>(syncStatus),
    };
  }

  LocalAttendanceEntry copyWith({
    String? enrollmentId,
    DateTime? date,
    String? sectionId,
    String? studentName,
    Value<String?> rollNumber = const Value.absent(),
    Value<String?> status = const Value.absent(),
    Value<String?> reason = const Value.absent(),
    int? serverRevision,
    String? syncStatus,
  }) => LocalAttendanceEntry(
    enrollmentId: enrollmentId ?? this.enrollmentId,
    date: date ?? this.date,
    sectionId: sectionId ?? this.sectionId,
    studentName: studentName ?? this.studentName,
    rollNumber: rollNumber.present ? rollNumber.value : this.rollNumber,
    status: status.present ? status.value : this.status,
    reason: reason.present ? reason.value : this.reason,
    serverRevision: serverRevision ?? this.serverRevision,
    syncStatus: syncStatus ?? this.syncStatus,
  );
  LocalAttendanceEntry copyWithCompanion(LocalAttendanceEntriesCompanion data) {
    return LocalAttendanceEntry(
      enrollmentId: data.enrollmentId.present
          ? data.enrollmentId.value
          : this.enrollmentId,
      date: data.date.present ? data.date.value : this.date,
      sectionId: data.sectionId.present ? data.sectionId.value : this.sectionId,
      studentName: data.studentName.present
          ? data.studentName.value
          : this.studentName,
      rollNumber: data.rollNumber.present
          ? data.rollNumber.value
          : this.rollNumber,
      status: data.status.present ? data.status.value : this.status,
      reason: data.reason.present ? data.reason.value : this.reason,
      serverRevision: data.serverRevision.present
          ? data.serverRevision.value
          : this.serverRevision,
      syncStatus: data.syncStatus.present
          ? data.syncStatus.value
          : this.syncStatus,
    );
  }

  @override
  String toString() {
    return (StringBuffer('LocalAttendanceEntry(')
          ..write('enrollmentId: $enrollmentId, ')
          ..write('date: $date, ')
          ..write('sectionId: $sectionId, ')
          ..write('studentName: $studentName, ')
          ..write('rollNumber: $rollNumber, ')
          ..write('status: $status, ')
          ..write('reason: $reason, ')
          ..write('serverRevision: $serverRevision, ')
          ..write('syncStatus: $syncStatus')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
    enrollmentId,
    date,
    sectionId,
    studentName,
    rollNumber,
    status,
    reason,
    serverRevision,
    syncStatus,
  );
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is LocalAttendanceEntry &&
          other.enrollmentId == this.enrollmentId &&
          other.date == this.date &&
          other.sectionId == this.sectionId &&
          other.studentName == this.studentName &&
          other.rollNumber == this.rollNumber &&
          other.status == this.status &&
          other.reason == this.reason &&
          other.serverRevision == this.serverRevision &&
          other.syncStatus == this.syncStatus);
}

class LocalAttendanceEntriesCompanion
    extends UpdateCompanion<LocalAttendanceEntry> {
  final Value<String> enrollmentId;
  final Value<DateTime> date;
  final Value<String> sectionId;
  final Value<String> studentName;
  final Value<String?> rollNumber;
  final Value<String?> status;
  final Value<String?> reason;
  final Value<int> serverRevision;
  final Value<String> syncStatus;
  final Value<int> rowid;
  const LocalAttendanceEntriesCompanion({
    this.enrollmentId = const Value.absent(),
    this.date = const Value.absent(),
    this.sectionId = const Value.absent(),
    this.studentName = const Value.absent(),
    this.rollNumber = const Value.absent(),
    this.status = const Value.absent(),
    this.reason = const Value.absent(),
    this.serverRevision = const Value.absent(),
    this.syncStatus = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  LocalAttendanceEntriesCompanion.insert({
    required String enrollmentId,
    required DateTime date,
    required String sectionId,
    required String studentName,
    this.rollNumber = const Value.absent(),
    this.status = const Value.absent(),
    this.reason = const Value.absent(),
    this.serverRevision = const Value.absent(),
    this.syncStatus = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : enrollmentId = Value(enrollmentId),
       date = Value(date),
       sectionId = Value(sectionId),
       studentName = Value(studentName);
  static Insertable<LocalAttendanceEntry> custom({
    Expression<String>? enrollmentId,
    Expression<DateTime>? date,
    Expression<String>? sectionId,
    Expression<String>? studentName,
    Expression<String>? rollNumber,
    Expression<String>? status,
    Expression<String>? reason,
    Expression<int>? serverRevision,
    Expression<String>? syncStatus,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (enrollmentId != null) 'enrollment_id': enrollmentId,
      if (date != null) 'date': date,
      if (sectionId != null) 'section_id': sectionId,
      if (studentName != null) 'student_name': studentName,
      if (rollNumber != null) 'roll_number': rollNumber,
      if (status != null) 'status': status,
      if (reason != null) 'reason': reason,
      if (serverRevision != null) 'server_revision': serverRevision,
      if (syncStatus != null) 'sync_status': syncStatus,
      if (rowid != null) 'rowid': rowid,
    });
  }

  LocalAttendanceEntriesCompanion copyWith({
    Value<String>? enrollmentId,
    Value<DateTime>? date,
    Value<String>? sectionId,
    Value<String>? studentName,
    Value<String?>? rollNumber,
    Value<String?>? status,
    Value<String?>? reason,
    Value<int>? serverRevision,
    Value<String>? syncStatus,
    Value<int>? rowid,
  }) {
    return LocalAttendanceEntriesCompanion(
      enrollmentId: enrollmentId ?? this.enrollmentId,
      date: date ?? this.date,
      sectionId: sectionId ?? this.sectionId,
      studentName: studentName ?? this.studentName,
      rollNumber: rollNumber ?? this.rollNumber,
      status: status ?? this.status,
      reason: reason ?? this.reason,
      serverRevision: serverRevision ?? this.serverRevision,
      syncStatus: syncStatus ?? this.syncStatus,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (enrollmentId.present) {
      map['enrollment_id'] = Variable<String>(enrollmentId.value);
    }
    if (date.present) {
      map['date'] = Variable<DateTime>(date.value);
    }
    if (sectionId.present) {
      map['section_id'] = Variable<String>(sectionId.value);
    }
    if (studentName.present) {
      map['student_name'] = Variable<String>(studentName.value);
    }
    if (rollNumber.present) {
      map['roll_number'] = Variable<String>(rollNumber.value);
    }
    if (status.present) {
      map['status'] = Variable<String>(status.value);
    }
    if (reason.present) {
      map['reason'] = Variable<String>(reason.value);
    }
    if (serverRevision.present) {
      map['server_revision'] = Variable<int>(serverRevision.value);
    }
    if (syncStatus.present) {
      map['sync_status'] = Variable<String>(syncStatus.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('LocalAttendanceEntriesCompanion(')
          ..write('enrollmentId: $enrollmentId, ')
          ..write('date: $date, ')
          ..write('sectionId: $sectionId, ')
          ..write('studentName: $studentName, ')
          ..write('rollNumber: $rollNumber, ')
          ..write('status: $status, ')
          ..write('reason: $reason, ')
          ..write('serverRevision: $serverRevision, ')
          ..write('syncStatus: $syncStatus, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $OutboxEntriesTable extends OutboxEntries
    with TableInfo<$OutboxEntriesTable, OutboxEntry> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $OutboxEntriesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<int> id = GeneratedColumn<int>(
    'id',
    aliasedName,
    false,
    hasAutoIncrement: true,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultConstraints: GeneratedColumn.constraintIsAlways(
      'PRIMARY KEY AUTOINCREMENT',
    ),
  );
  static const VerificationMeta _enrollmentIdMeta = const VerificationMeta(
    'enrollmentId',
  );
  @override
  late final GeneratedColumn<String> enrollmentId = GeneratedColumn<String>(
    'enrollment_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _dateMeta = const VerificationMeta('date');
  @override
  late final GeneratedColumn<DateTime> date = GeneratedColumn<DateTime>(
    'date',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _sectionIdMeta = const VerificationMeta(
    'sectionId',
  );
  @override
  late final GeneratedColumn<String> sectionId = GeneratedColumn<String>(
    'section_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _statusMeta = const VerificationMeta('status');
  @override
  late final GeneratedColumn<String> status = GeneratedColumn<String>(
    'status',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _reasonMeta = const VerificationMeta('reason');
  @override
  late final GeneratedColumn<String> reason = GeneratedColumn<String>(
    'reason',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _localCounterMeta = const VerificationMeta(
    'localCounter',
  );
  @override
  late final GeneratedColumn<int> localCounter = GeneratedColumn<int>(
    'local_counter',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _clientTimestampMeta = const VerificationMeta(
    'clientTimestamp',
  );
  @override
  late final GeneratedColumn<DateTime> clientTimestamp =
      GeneratedColumn<DateTime>(
        'client_timestamp',
        aliasedName,
        false,
        type: DriftSqlType.dateTime,
        requiredDuringInsert: true,
      );
  static const VerificationMeta _baseRevisionMeta = const VerificationMeta(
    'baseRevision',
  );
  @override
  late final GeneratedColumn<int> baseRevision = GeneratedColumn<int>(
    'base_revision',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _createdAtMeta = const VerificationMeta(
    'createdAt',
  );
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
    'created_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: false,
    defaultValue: currentDateAndTime,
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    enrollmentId,
    date,
    sectionId,
    status,
    reason,
    localCounter,
    clientTimestamp,
    baseRevision,
    createdAt,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'outbox_entries';
  @override
  VerificationContext validateIntegrity(
    Insertable<OutboxEntry> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    }
    if (data.containsKey('enrollment_id')) {
      context.handle(
        _enrollmentIdMeta,
        enrollmentId.isAcceptableOrUnknown(
          data['enrollment_id']!,
          _enrollmentIdMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_enrollmentIdMeta);
    }
    if (data.containsKey('date')) {
      context.handle(
        _dateMeta,
        date.isAcceptableOrUnknown(data['date']!, _dateMeta),
      );
    } else if (isInserting) {
      context.missing(_dateMeta);
    }
    if (data.containsKey('section_id')) {
      context.handle(
        _sectionIdMeta,
        sectionId.isAcceptableOrUnknown(data['section_id']!, _sectionIdMeta),
      );
    } else if (isInserting) {
      context.missing(_sectionIdMeta);
    }
    if (data.containsKey('status')) {
      context.handle(
        _statusMeta,
        status.isAcceptableOrUnknown(data['status']!, _statusMeta),
      );
    } else if (isInserting) {
      context.missing(_statusMeta);
    }
    if (data.containsKey('reason')) {
      context.handle(
        _reasonMeta,
        reason.isAcceptableOrUnknown(data['reason']!, _reasonMeta),
      );
    }
    if (data.containsKey('local_counter')) {
      context.handle(
        _localCounterMeta,
        localCounter.isAcceptableOrUnknown(
          data['local_counter']!,
          _localCounterMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_localCounterMeta);
    }
    if (data.containsKey('client_timestamp')) {
      context.handle(
        _clientTimestampMeta,
        clientTimestamp.isAcceptableOrUnknown(
          data['client_timestamp']!,
          _clientTimestampMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_clientTimestampMeta);
    }
    if (data.containsKey('base_revision')) {
      context.handle(
        _baseRevisionMeta,
        baseRevision.isAcceptableOrUnknown(
          data['base_revision']!,
          _baseRevisionMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_baseRevisionMeta);
    }
    if (data.containsKey('created_at')) {
      context.handle(
        _createdAtMeta,
        createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta),
      );
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  OutboxEntry map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return OutboxEntry(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}id'],
      )!,
      enrollmentId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}enrollment_id'],
      )!,
      date: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}date'],
      )!,
      sectionId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}section_id'],
      )!,
      status: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}status'],
      )!,
      reason: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}reason'],
      ),
      localCounter: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}local_counter'],
      )!,
      clientTimestamp: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}client_timestamp'],
      )!,
      baseRevision: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}base_revision'],
      )!,
      createdAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}created_at'],
      )!,
    );
  }

  @override
  $OutboxEntriesTable createAlias(String alias) {
    return $OutboxEntriesTable(attachedDatabase, alias);
  }
}

class OutboxEntry extends DataClass implements Insertable<OutboxEntry> {
  final int id;
  final String enrollmentId;
  final DateTime date;
  final String sectionId;
  final String status;
  final String? reason;
  final int localCounter;
  final DateTime clientTimestamp;
  final int baseRevision;
  final DateTime createdAt;
  const OutboxEntry({
    required this.id,
    required this.enrollmentId,
    required this.date,
    required this.sectionId,
    required this.status,
    this.reason,
    required this.localCounter,
    required this.clientTimestamp,
    required this.baseRevision,
    required this.createdAt,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<int>(id);
    map['enrollment_id'] = Variable<String>(enrollmentId);
    map['date'] = Variable<DateTime>(date);
    map['section_id'] = Variable<String>(sectionId);
    map['status'] = Variable<String>(status);
    if (!nullToAbsent || reason != null) {
      map['reason'] = Variable<String>(reason);
    }
    map['local_counter'] = Variable<int>(localCounter);
    map['client_timestamp'] = Variable<DateTime>(clientTimestamp);
    map['base_revision'] = Variable<int>(baseRevision);
    map['created_at'] = Variable<DateTime>(createdAt);
    return map;
  }

  OutboxEntriesCompanion toCompanion(bool nullToAbsent) {
    return OutboxEntriesCompanion(
      id: Value(id),
      enrollmentId: Value(enrollmentId),
      date: Value(date),
      sectionId: Value(sectionId),
      status: Value(status),
      reason: reason == null && nullToAbsent
          ? const Value.absent()
          : Value(reason),
      localCounter: Value(localCounter),
      clientTimestamp: Value(clientTimestamp),
      baseRevision: Value(baseRevision),
      createdAt: Value(createdAt),
    );
  }

  factory OutboxEntry.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return OutboxEntry(
      id: serializer.fromJson<int>(json['id']),
      enrollmentId: serializer.fromJson<String>(json['enrollmentId']),
      date: serializer.fromJson<DateTime>(json['date']),
      sectionId: serializer.fromJson<String>(json['sectionId']),
      status: serializer.fromJson<String>(json['status']),
      reason: serializer.fromJson<String?>(json['reason']),
      localCounter: serializer.fromJson<int>(json['localCounter']),
      clientTimestamp: serializer.fromJson<DateTime>(json['clientTimestamp']),
      baseRevision: serializer.fromJson<int>(json['baseRevision']),
      createdAt: serializer.fromJson<DateTime>(json['createdAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<int>(id),
      'enrollmentId': serializer.toJson<String>(enrollmentId),
      'date': serializer.toJson<DateTime>(date),
      'sectionId': serializer.toJson<String>(sectionId),
      'status': serializer.toJson<String>(status),
      'reason': serializer.toJson<String?>(reason),
      'localCounter': serializer.toJson<int>(localCounter),
      'clientTimestamp': serializer.toJson<DateTime>(clientTimestamp),
      'baseRevision': serializer.toJson<int>(baseRevision),
      'createdAt': serializer.toJson<DateTime>(createdAt),
    };
  }

  OutboxEntry copyWith({
    int? id,
    String? enrollmentId,
    DateTime? date,
    String? sectionId,
    String? status,
    Value<String?> reason = const Value.absent(),
    int? localCounter,
    DateTime? clientTimestamp,
    int? baseRevision,
    DateTime? createdAt,
  }) => OutboxEntry(
    id: id ?? this.id,
    enrollmentId: enrollmentId ?? this.enrollmentId,
    date: date ?? this.date,
    sectionId: sectionId ?? this.sectionId,
    status: status ?? this.status,
    reason: reason.present ? reason.value : this.reason,
    localCounter: localCounter ?? this.localCounter,
    clientTimestamp: clientTimestamp ?? this.clientTimestamp,
    baseRevision: baseRevision ?? this.baseRevision,
    createdAt: createdAt ?? this.createdAt,
  );
  OutboxEntry copyWithCompanion(OutboxEntriesCompanion data) {
    return OutboxEntry(
      id: data.id.present ? data.id.value : this.id,
      enrollmentId: data.enrollmentId.present
          ? data.enrollmentId.value
          : this.enrollmentId,
      date: data.date.present ? data.date.value : this.date,
      sectionId: data.sectionId.present ? data.sectionId.value : this.sectionId,
      status: data.status.present ? data.status.value : this.status,
      reason: data.reason.present ? data.reason.value : this.reason,
      localCounter: data.localCounter.present
          ? data.localCounter.value
          : this.localCounter,
      clientTimestamp: data.clientTimestamp.present
          ? data.clientTimestamp.value
          : this.clientTimestamp,
      baseRevision: data.baseRevision.present
          ? data.baseRevision.value
          : this.baseRevision,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('OutboxEntry(')
          ..write('id: $id, ')
          ..write('enrollmentId: $enrollmentId, ')
          ..write('date: $date, ')
          ..write('sectionId: $sectionId, ')
          ..write('status: $status, ')
          ..write('reason: $reason, ')
          ..write('localCounter: $localCounter, ')
          ..write('clientTimestamp: $clientTimestamp, ')
          ..write('baseRevision: $baseRevision, ')
          ..write('createdAt: $createdAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
    id,
    enrollmentId,
    date,
    sectionId,
    status,
    reason,
    localCounter,
    clientTimestamp,
    baseRevision,
    createdAt,
  );
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is OutboxEntry &&
          other.id == this.id &&
          other.enrollmentId == this.enrollmentId &&
          other.date == this.date &&
          other.sectionId == this.sectionId &&
          other.status == this.status &&
          other.reason == this.reason &&
          other.localCounter == this.localCounter &&
          other.clientTimestamp == this.clientTimestamp &&
          other.baseRevision == this.baseRevision &&
          other.createdAt == this.createdAt);
}

class OutboxEntriesCompanion extends UpdateCompanion<OutboxEntry> {
  final Value<int> id;
  final Value<String> enrollmentId;
  final Value<DateTime> date;
  final Value<String> sectionId;
  final Value<String> status;
  final Value<String?> reason;
  final Value<int> localCounter;
  final Value<DateTime> clientTimestamp;
  final Value<int> baseRevision;
  final Value<DateTime> createdAt;
  const OutboxEntriesCompanion({
    this.id = const Value.absent(),
    this.enrollmentId = const Value.absent(),
    this.date = const Value.absent(),
    this.sectionId = const Value.absent(),
    this.status = const Value.absent(),
    this.reason = const Value.absent(),
    this.localCounter = const Value.absent(),
    this.clientTimestamp = const Value.absent(),
    this.baseRevision = const Value.absent(),
    this.createdAt = const Value.absent(),
  });
  OutboxEntriesCompanion.insert({
    this.id = const Value.absent(),
    required String enrollmentId,
    required DateTime date,
    required String sectionId,
    required String status,
    this.reason = const Value.absent(),
    required int localCounter,
    required DateTime clientTimestamp,
    required int baseRevision,
    this.createdAt = const Value.absent(),
  }) : enrollmentId = Value(enrollmentId),
       date = Value(date),
       sectionId = Value(sectionId),
       status = Value(status),
       localCounter = Value(localCounter),
       clientTimestamp = Value(clientTimestamp),
       baseRevision = Value(baseRevision);
  static Insertable<OutboxEntry> custom({
    Expression<int>? id,
    Expression<String>? enrollmentId,
    Expression<DateTime>? date,
    Expression<String>? sectionId,
    Expression<String>? status,
    Expression<String>? reason,
    Expression<int>? localCounter,
    Expression<DateTime>? clientTimestamp,
    Expression<int>? baseRevision,
    Expression<DateTime>? createdAt,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (enrollmentId != null) 'enrollment_id': enrollmentId,
      if (date != null) 'date': date,
      if (sectionId != null) 'section_id': sectionId,
      if (status != null) 'status': status,
      if (reason != null) 'reason': reason,
      if (localCounter != null) 'local_counter': localCounter,
      if (clientTimestamp != null) 'client_timestamp': clientTimestamp,
      if (baseRevision != null) 'base_revision': baseRevision,
      if (createdAt != null) 'created_at': createdAt,
    });
  }

  OutboxEntriesCompanion copyWith({
    Value<int>? id,
    Value<String>? enrollmentId,
    Value<DateTime>? date,
    Value<String>? sectionId,
    Value<String>? status,
    Value<String?>? reason,
    Value<int>? localCounter,
    Value<DateTime>? clientTimestamp,
    Value<int>? baseRevision,
    Value<DateTime>? createdAt,
  }) {
    return OutboxEntriesCompanion(
      id: id ?? this.id,
      enrollmentId: enrollmentId ?? this.enrollmentId,
      date: date ?? this.date,
      sectionId: sectionId ?? this.sectionId,
      status: status ?? this.status,
      reason: reason ?? this.reason,
      localCounter: localCounter ?? this.localCounter,
      clientTimestamp: clientTimestamp ?? this.clientTimestamp,
      baseRevision: baseRevision ?? this.baseRevision,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<int>(id.value);
    }
    if (enrollmentId.present) {
      map['enrollment_id'] = Variable<String>(enrollmentId.value);
    }
    if (date.present) {
      map['date'] = Variable<DateTime>(date.value);
    }
    if (sectionId.present) {
      map['section_id'] = Variable<String>(sectionId.value);
    }
    if (status.present) {
      map['status'] = Variable<String>(status.value);
    }
    if (reason.present) {
      map['reason'] = Variable<String>(reason.value);
    }
    if (localCounter.present) {
      map['local_counter'] = Variable<int>(localCounter.value);
    }
    if (clientTimestamp.present) {
      map['client_timestamp'] = Variable<DateTime>(clientTimestamp.value);
    }
    if (baseRevision.present) {
      map['base_revision'] = Variable<int>(baseRevision.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('OutboxEntriesCompanion(')
          ..write('id: $id, ')
          ..write('enrollmentId: $enrollmentId, ')
          ..write('date: $date, ')
          ..write('sectionId: $sectionId, ')
          ..write('status: $status, ')
          ..write('reason: $reason, ')
          ..write('localCounter: $localCounter, ')
          ..write('clientTimestamp: $clientTimestamp, ')
          ..write('baseRevision: $baseRevision, ')
          ..write('createdAt: $createdAt')
          ..write(')'))
        .toString();
  }
}

class $DeviceStateTable extends DeviceState
    with TableInfo<$DeviceStateTable, DeviceStateData> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $DeviceStateTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _keyMeta = const VerificationMeta('key');
  @override
  late final GeneratedColumn<String> key = GeneratedColumn<String>(
    'key',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _valueMeta = const VerificationMeta('value');
  @override
  late final GeneratedColumn<String> value = GeneratedColumn<String>(
    'value',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [key, value];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'device_state';
  @override
  VerificationContext validateIntegrity(
    Insertable<DeviceStateData> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('key')) {
      context.handle(
        _keyMeta,
        key.isAcceptableOrUnknown(data['key']!, _keyMeta),
      );
    } else if (isInserting) {
      context.missing(_keyMeta);
    }
    if (data.containsKey('value')) {
      context.handle(
        _valueMeta,
        value.isAcceptableOrUnknown(data['value']!, _valueMeta),
      );
    } else if (isInserting) {
      context.missing(_valueMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {key};
  @override
  DeviceStateData map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return DeviceStateData(
      key: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}key'],
      )!,
      value: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}value'],
      )!,
    );
  }

  @override
  $DeviceStateTable createAlias(String alias) {
    return $DeviceStateTable(attachedDatabase, alias);
  }
}

class DeviceStateData extends DataClass implements Insertable<DeviceStateData> {
  final String key;
  final String value;
  const DeviceStateData({required this.key, required this.value});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['key'] = Variable<String>(key);
    map['value'] = Variable<String>(value);
    return map;
  }

  DeviceStateCompanion toCompanion(bool nullToAbsent) {
    return DeviceStateCompanion(key: Value(key), value: Value(value));
  }

  factory DeviceStateData.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return DeviceStateData(
      key: serializer.fromJson<String>(json['key']),
      value: serializer.fromJson<String>(json['value']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'key': serializer.toJson<String>(key),
      'value': serializer.toJson<String>(value),
    };
  }

  DeviceStateData copyWith({String? key, String? value}) =>
      DeviceStateData(key: key ?? this.key, value: value ?? this.value);
  DeviceStateData copyWithCompanion(DeviceStateCompanion data) {
    return DeviceStateData(
      key: data.key.present ? data.key.value : this.key,
      value: data.value.present ? data.value.value : this.value,
    );
  }

  @override
  String toString() {
    return (StringBuffer('DeviceStateData(')
          ..write('key: $key, ')
          ..write('value: $value')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(key, value);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is DeviceStateData &&
          other.key == this.key &&
          other.value == this.value);
}

class DeviceStateCompanion extends UpdateCompanion<DeviceStateData> {
  final Value<String> key;
  final Value<String> value;
  final Value<int> rowid;
  const DeviceStateCompanion({
    this.key = const Value.absent(),
    this.value = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  DeviceStateCompanion.insert({
    required String key,
    required String value,
    this.rowid = const Value.absent(),
  }) : key = Value(key),
       value = Value(value);
  static Insertable<DeviceStateData> custom({
    Expression<String>? key,
    Expression<String>? value,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (key != null) 'key': key,
      if (value != null) 'value': value,
      if (rowid != null) 'rowid': rowid,
    });
  }

  DeviceStateCompanion copyWith({
    Value<String>? key,
    Value<String>? value,
    Value<int>? rowid,
  }) {
    return DeviceStateCompanion(
      key: key ?? this.key,
      value: value ?? this.value,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (key.present) {
      map['key'] = Variable<String>(key.value);
    }
    if (value.present) {
      map['value'] = Variable<String>(value.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('DeviceStateCompanion(')
          ..write('key: $key, ')
          ..write('value: $value, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

abstract class _$AppDatabase extends GeneratedDatabase {
  _$AppDatabase(QueryExecutor e) : super(e);
  $AppDatabaseManager get managers => $AppDatabaseManager(this);
  late final $LocalAttendanceEntriesTable localAttendanceEntries =
      $LocalAttendanceEntriesTable(this);
  late final $OutboxEntriesTable outboxEntries = $OutboxEntriesTable(this);
  late final $DeviceStateTable deviceState = $DeviceStateTable(this);
  @override
  Iterable<TableInfo<Table, Object?>> get allTables =>
      allSchemaEntities.whereType<TableInfo<Table, Object?>>();
  @override
  List<DatabaseSchemaEntity> get allSchemaEntities => [
    localAttendanceEntries,
    outboxEntries,
    deviceState,
  ];
}

typedef $$LocalAttendanceEntriesTableCreateCompanionBuilder =
    LocalAttendanceEntriesCompanion Function({
      required String enrollmentId,
      required DateTime date,
      required String sectionId,
      required String studentName,
      Value<String?> rollNumber,
      Value<String?> status,
      Value<String?> reason,
      Value<int> serverRevision,
      Value<String> syncStatus,
      Value<int> rowid,
    });
typedef $$LocalAttendanceEntriesTableUpdateCompanionBuilder =
    LocalAttendanceEntriesCompanion Function({
      Value<String> enrollmentId,
      Value<DateTime> date,
      Value<String> sectionId,
      Value<String> studentName,
      Value<String?> rollNumber,
      Value<String?> status,
      Value<String?> reason,
      Value<int> serverRevision,
      Value<String> syncStatus,
      Value<int> rowid,
    });

class $$LocalAttendanceEntriesTableFilterComposer
    extends Composer<_$AppDatabase, $LocalAttendanceEntriesTable> {
  $$LocalAttendanceEntriesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get enrollmentId => $composableBuilder(
    column: $table.enrollmentId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get date => $composableBuilder(
    column: $table.date,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get sectionId => $composableBuilder(
    column: $table.sectionId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get studentName => $composableBuilder(
    column: $table.studentName,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get rollNumber => $composableBuilder(
    column: $table.rollNumber,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get reason => $composableBuilder(
    column: $table.reason,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get serverRevision => $composableBuilder(
    column: $table.serverRevision,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get syncStatus => $composableBuilder(
    column: $table.syncStatus,
    builder: (column) => ColumnFilters(column),
  );
}

class $$LocalAttendanceEntriesTableOrderingComposer
    extends Composer<_$AppDatabase, $LocalAttendanceEntriesTable> {
  $$LocalAttendanceEntriesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get enrollmentId => $composableBuilder(
    column: $table.enrollmentId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get date => $composableBuilder(
    column: $table.date,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get sectionId => $composableBuilder(
    column: $table.sectionId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get studentName => $composableBuilder(
    column: $table.studentName,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get rollNumber => $composableBuilder(
    column: $table.rollNumber,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get reason => $composableBuilder(
    column: $table.reason,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get serverRevision => $composableBuilder(
    column: $table.serverRevision,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get syncStatus => $composableBuilder(
    column: $table.syncStatus,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$LocalAttendanceEntriesTableAnnotationComposer
    extends Composer<_$AppDatabase, $LocalAttendanceEntriesTable> {
  $$LocalAttendanceEntriesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get enrollmentId => $composableBuilder(
    column: $table.enrollmentId,
    builder: (column) => column,
  );

  GeneratedColumn<DateTime> get date =>
      $composableBuilder(column: $table.date, builder: (column) => column);

  GeneratedColumn<String> get sectionId =>
      $composableBuilder(column: $table.sectionId, builder: (column) => column);

  GeneratedColumn<String> get studentName => $composableBuilder(
    column: $table.studentName,
    builder: (column) => column,
  );

  GeneratedColumn<String> get rollNumber => $composableBuilder(
    column: $table.rollNumber,
    builder: (column) => column,
  );

  GeneratedColumn<String> get status =>
      $composableBuilder(column: $table.status, builder: (column) => column);

  GeneratedColumn<String> get reason =>
      $composableBuilder(column: $table.reason, builder: (column) => column);

  GeneratedColumn<int> get serverRevision => $composableBuilder(
    column: $table.serverRevision,
    builder: (column) => column,
  );

  GeneratedColumn<String> get syncStatus => $composableBuilder(
    column: $table.syncStatus,
    builder: (column) => column,
  );
}

class $$LocalAttendanceEntriesTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $LocalAttendanceEntriesTable,
          LocalAttendanceEntry,
          $$LocalAttendanceEntriesTableFilterComposer,
          $$LocalAttendanceEntriesTableOrderingComposer,
          $$LocalAttendanceEntriesTableAnnotationComposer,
          $$LocalAttendanceEntriesTableCreateCompanionBuilder,
          $$LocalAttendanceEntriesTableUpdateCompanionBuilder,
          (
            LocalAttendanceEntry,
            BaseReferences<
              _$AppDatabase,
              $LocalAttendanceEntriesTable,
              LocalAttendanceEntry
            >,
          ),
          LocalAttendanceEntry,
          PrefetchHooks Function()
        > {
  $$LocalAttendanceEntriesTableTableManager(
    _$AppDatabase db,
    $LocalAttendanceEntriesTable table,
  ) : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$LocalAttendanceEntriesTableFilterComposer(
                $db: db,
                $table: table,
              ),
          createOrderingComposer: () =>
              $$LocalAttendanceEntriesTableOrderingComposer(
                $db: db,
                $table: table,
              ),
          createComputedFieldComposer: () =>
              $$LocalAttendanceEntriesTableAnnotationComposer(
                $db: db,
                $table: table,
              ),
          updateCompanionCallback:
              ({
                Value<String> enrollmentId = const Value.absent(),
                Value<DateTime> date = const Value.absent(),
                Value<String> sectionId = const Value.absent(),
                Value<String> studentName = const Value.absent(),
                Value<String?> rollNumber = const Value.absent(),
                Value<String?> status = const Value.absent(),
                Value<String?> reason = const Value.absent(),
                Value<int> serverRevision = const Value.absent(),
                Value<String> syncStatus = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => LocalAttendanceEntriesCompanion(
                enrollmentId: enrollmentId,
                date: date,
                sectionId: sectionId,
                studentName: studentName,
                rollNumber: rollNumber,
                status: status,
                reason: reason,
                serverRevision: serverRevision,
                syncStatus: syncStatus,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String enrollmentId,
                required DateTime date,
                required String sectionId,
                required String studentName,
                Value<String?> rollNumber = const Value.absent(),
                Value<String?> status = const Value.absent(),
                Value<String?> reason = const Value.absent(),
                Value<int> serverRevision = const Value.absent(),
                Value<String> syncStatus = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => LocalAttendanceEntriesCompanion.insert(
                enrollmentId: enrollmentId,
                date: date,
                sectionId: sectionId,
                studentName: studentName,
                rollNumber: rollNumber,
                status: status,
                reason: reason,
                serverRevision: serverRevision,
                syncStatus: syncStatus,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$LocalAttendanceEntriesTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $LocalAttendanceEntriesTable,
      LocalAttendanceEntry,
      $$LocalAttendanceEntriesTableFilterComposer,
      $$LocalAttendanceEntriesTableOrderingComposer,
      $$LocalAttendanceEntriesTableAnnotationComposer,
      $$LocalAttendanceEntriesTableCreateCompanionBuilder,
      $$LocalAttendanceEntriesTableUpdateCompanionBuilder,
      (
        LocalAttendanceEntry,
        BaseReferences<
          _$AppDatabase,
          $LocalAttendanceEntriesTable,
          LocalAttendanceEntry
        >,
      ),
      LocalAttendanceEntry,
      PrefetchHooks Function()
    >;
typedef $$OutboxEntriesTableCreateCompanionBuilder =
    OutboxEntriesCompanion Function({
      Value<int> id,
      required String enrollmentId,
      required DateTime date,
      required String sectionId,
      required String status,
      Value<String?> reason,
      required int localCounter,
      required DateTime clientTimestamp,
      required int baseRevision,
      Value<DateTime> createdAt,
    });
typedef $$OutboxEntriesTableUpdateCompanionBuilder =
    OutboxEntriesCompanion Function({
      Value<int> id,
      Value<String> enrollmentId,
      Value<DateTime> date,
      Value<String> sectionId,
      Value<String> status,
      Value<String?> reason,
      Value<int> localCounter,
      Value<DateTime> clientTimestamp,
      Value<int> baseRevision,
      Value<DateTime> createdAt,
    });

class $$OutboxEntriesTableFilterComposer
    extends Composer<_$AppDatabase, $OutboxEntriesTable> {
  $$OutboxEntriesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<int> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get enrollmentId => $composableBuilder(
    column: $table.enrollmentId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get date => $composableBuilder(
    column: $table.date,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get sectionId => $composableBuilder(
    column: $table.sectionId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get reason => $composableBuilder(
    column: $table.reason,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get localCounter => $composableBuilder(
    column: $table.localCounter,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get clientTimestamp => $composableBuilder(
    column: $table.clientTimestamp,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get baseRevision => $composableBuilder(
    column: $table.baseRevision,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnFilters(column),
  );
}

class $$OutboxEntriesTableOrderingComposer
    extends Composer<_$AppDatabase, $OutboxEntriesTable> {
  $$OutboxEntriesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<int> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get enrollmentId => $composableBuilder(
    column: $table.enrollmentId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get date => $composableBuilder(
    column: $table.date,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get sectionId => $composableBuilder(
    column: $table.sectionId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get reason => $composableBuilder(
    column: $table.reason,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get localCounter => $composableBuilder(
    column: $table.localCounter,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get clientTimestamp => $composableBuilder(
    column: $table.clientTimestamp,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get baseRevision => $composableBuilder(
    column: $table.baseRevision,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$OutboxEntriesTableAnnotationComposer
    extends Composer<_$AppDatabase, $OutboxEntriesTable> {
  $$OutboxEntriesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<int> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get enrollmentId => $composableBuilder(
    column: $table.enrollmentId,
    builder: (column) => column,
  );

  GeneratedColumn<DateTime> get date =>
      $composableBuilder(column: $table.date, builder: (column) => column);

  GeneratedColumn<String> get sectionId =>
      $composableBuilder(column: $table.sectionId, builder: (column) => column);

  GeneratedColumn<String> get status =>
      $composableBuilder(column: $table.status, builder: (column) => column);

  GeneratedColumn<String> get reason =>
      $composableBuilder(column: $table.reason, builder: (column) => column);

  GeneratedColumn<int> get localCounter => $composableBuilder(
    column: $table.localCounter,
    builder: (column) => column,
  );

  GeneratedColumn<DateTime> get clientTimestamp => $composableBuilder(
    column: $table.clientTimestamp,
    builder: (column) => column,
  );

  GeneratedColumn<int> get baseRevision => $composableBuilder(
    column: $table.baseRevision,
    builder: (column) => column,
  );

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);
}

class $$OutboxEntriesTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $OutboxEntriesTable,
          OutboxEntry,
          $$OutboxEntriesTableFilterComposer,
          $$OutboxEntriesTableOrderingComposer,
          $$OutboxEntriesTableAnnotationComposer,
          $$OutboxEntriesTableCreateCompanionBuilder,
          $$OutboxEntriesTableUpdateCompanionBuilder,
          (
            OutboxEntry,
            BaseReferences<_$AppDatabase, $OutboxEntriesTable, OutboxEntry>,
          ),
          OutboxEntry,
          PrefetchHooks Function()
        > {
  $$OutboxEntriesTableTableManager(_$AppDatabase db, $OutboxEntriesTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$OutboxEntriesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$OutboxEntriesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$OutboxEntriesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<int> id = const Value.absent(),
                Value<String> enrollmentId = const Value.absent(),
                Value<DateTime> date = const Value.absent(),
                Value<String> sectionId = const Value.absent(),
                Value<String> status = const Value.absent(),
                Value<String?> reason = const Value.absent(),
                Value<int> localCounter = const Value.absent(),
                Value<DateTime> clientTimestamp = const Value.absent(),
                Value<int> baseRevision = const Value.absent(),
                Value<DateTime> createdAt = const Value.absent(),
              }) => OutboxEntriesCompanion(
                id: id,
                enrollmentId: enrollmentId,
                date: date,
                sectionId: sectionId,
                status: status,
                reason: reason,
                localCounter: localCounter,
                clientTimestamp: clientTimestamp,
                baseRevision: baseRevision,
                createdAt: createdAt,
              ),
          createCompanionCallback:
              ({
                Value<int> id = const Value.absent(),
                required String enrollmentId,
                required DateTime date,
                required String sectionId,
                required String status,
                Value<String?> reason = const Value.absent(),
                required int localCounter,
                required DateTime clientTimestamp,
                required int baseRevision,
                Value<DateTime> createdAt = const Value.absent(),
              }) => OutboxEntriesCompanion.insert(
                id: id,
                enrollmentId: enrollmentId,
                date: date,
                sectionId: sectionId,
                status: status,
                reason: reason,
                localCounter: localCounter,
                clientTimestamp: clientTimestamp,
                baseRevision: baseRevision,
                createdAt: createdAt,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$OutboxEntriesTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $OutboxEntriesTable,
      OutboxEntry,
      $$OutboxEntriesTableFilterComposer,
      $$OutboxEntriesTableOrderingComposer,
      $$OutboxEntriesTableAnnotationComposer,
      $$OutboxEntriesTableCreateCompanionBuilder,
      $$OutboxEntriesTableUpdateCompanionBuilder,
      (
        OutboxEntry,
        BaseReferences<_$AppDatabase, $OutboxEntriesTable, OutboxEntry>,
      ),
      OutboxEntry,
      PrefetchHooks Function()
    >;
typedef $$DeviceStateTableCreateCompanionBuilder =
    DeviceStateCompanion Function({
      required String key,
      required String value,
      Value<int> rowid,
    });
typedef $$DeviceStateTableUpdateCompanionBuilder =
    DeviceStateCompanion Function({
      Value<String> key,
      Value<String> value,
      Value<int> rowid,
    });

class $$DeviceStateTableFilterComposer
    extends Composer<_$AppDatabase, $DeviceStateTable> {
  $$DeviceStateTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get key => $composableBuilder(
    column: $table.key,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get value => $composableBuilder(
    column: $table.value,
    builder: (column) => ColumnFilters(column),
  );
}

class $$DeviceStateTableOrderingComposer
    extends Composer<_$AppDatabase, $DeviceStateTable> {
  $$DeviceStateTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get key => $composableBuilder(
    column: $table.key,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get value => $composableBuilder(
    column: $table.value,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$DeviceStateTableAnnotationComposer
    extends Composer<_$AppDatabase, $DeviceStateTable> {
  $$DeviceStateTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get key =>
      $composableBuilder(column: $table.key, builder: (column) => column);

  GeneratedColumn<String> get value =>
      $composableBuilder(column: $table.value, builder: (column) => column);
}

class $$DeviceStateTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $DeviceStateTable,
          DeviceStateData,
          $$DeviceStateTableFilterComposer,
          $$DeviceStateTableOrderingComposer,
          $$DeviceStateTableAnnotationComposer,
          $$DeviceStateTableCreateCompanionBuilder,
          $$DeviceStateTableUpdateCompanionBuilder,
          (
            DeviceStateData,
            BaseReferences<_$AppDatabase, $DeviceStateTable, DeviceStateData>,
          ),
          DeviceStateData,
          PrefetchHooks Function()
        > {
  $$DeviceStateTableTableManager(_$AppDatabase db, $DeviceStateTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$DeviceStateTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$DeviceStateTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$DeviceStateTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> key = const Value.absent(),
                Value<String> value = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => DeviceStateCompanion(key: key, value: value, rowid: rowid),
          createCompanionCallback:
              ({
                required String key,
                required String value,
                Value<int> rowid = const Value.absent(),
              }) => DeviceStateCompanion.insert(
                key: key,
                value: value,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$DeviceStateTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $DeviceStateTable,
      DeviceStateData,
      $$DeviceStateTableFilterComposer,
      $$DeviceStateTableOrderingComposer,
      $$DeviceStateTableAnnotationComposer,
      $$DeviceStateTableCreateCompanionBuilder,
      $$DeviceStateTableUpdateCompanionBuilder,
      (
        DeviceStateData,
        BaseReferences<_$AppDatabase, $DeviceStateTable, DeviceStateData>,
      ),
      DeviceStateData,
      PrefetchHooks Function()
    >;

class $AppDatabaseManager {
  final _$AppDatabase _db;
  $AppDatabaseManager(this._db);
  $$LocalAttendanceEntriesTableTableManager get localAttendanceEntries =>
      $$LocalAttendanceEntriesTableTableManager(
        _db,
        _db.localAttendanceEntries,
      );
  $$OutboxEntriesTableTableManager get outboxEntries =>
      $$OutboxEntriesTableTableManager(_db, _db.outboxEntries);
  $$DeviceStateTableTableManager get deviceState =>
      $$DeviceStateTableTableManager(_db, _db.deviceState);
}
